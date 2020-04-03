package bot

import (
	"errors"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/karlseguin/ccache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MemberFetcher = &memberFetcher{
		fetching:    make(map[int64]*MemberFetchGuildQueue),
		notFetching: make(map[int64]*MemberFetchGuildQueue),
	}

	failedUsersCache = ccache.New(ccache.Configure())
)

// GetMember will either return a member from state or fetch one from the member fetcher and then put it in state
func GetMember(guildID, userID int64) (*dstate.MemberState, error) {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID)
	if cop != nil && cop.MemberSet {
		return cop, nil
	}

	result := <-MemberFetcher.RequestMember(guildID, userID, false)
	return result.Member, result.Err
}

// GetMembers is the same as GetMember but with multiple members
func GetMembers(guildID int64, userIDs ...int64) ([]*dstate.MemberState, error) {
	resultChan := make(chan *dstate.MemberState)
	for _, v := range userIDs {
		go func(id int64) {
			m, _ := GetMember(guildID, id)
			resultChan <- m
		}(v)
	}

	result := make([]*dstate.MemberState, 0, len(userIDs))
	for i := 0; i < len(userIDs); i++ {
		m := <-resultChan
		if m != nil {
			result = append(result, m)
		}
	}

	return result, nil
}

// GetMemberJoinedAt is the same as GetMember but it ensures the JoinedAt field is present
func GetMemberJoinedAt(guildID, userID int64) (*dstate.MemberState, error) {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID)
	if cop != nil && cop.MemberSet && !cop.JoinedAt.IsZero() {
		return cop, nil
	}

	result := <-MemberFetcher.RequestMember(guildID, userID, true)
	return result.Member, result.Err
}

// memberFetcher handles a per guild queue for fetching members
// This is probably overkill as the root cause for the flood of member requests (state being flushed upon guilds becoming unavailble) was fixed
// But it's here, for better or for worse
type memberFetcher struct {
	sync.RWMutex

	// Queue of guilds to user id's to fetch
	fetching    map[int64]*MemberFetchGuildQueue
	notFetching map[int64]*MemberFetchGuildQueue

	// Signal to run immediately
	RunChan chan bool
	Stop    chan bool
}

type MemberFetchGuildQueue struct {
	Queue []*MemberFetchRequest
}

type MemberFetchRequest struct {
	Member          int64
	Guild           int64
	NeedJoinedAt    bool
	WaitingChannels []chan MemberFetchResult
}

func (req *MemberFetchRequest) sendResult(result MemberFetchResult) {
	for _, ch := range req.WaitingChannels {
		go func(channel chan MemberFetchResult) {
			channel <- result
		}(ch)
	}
}

type MemberFetchResult struct {
	Err    error
	Member *dstate.MemberState
}

func (m *memberFetcher) RequestMember(guildID, userID int64, requireJoinedAt bool) <-chan MemberFetchResult {
	m.Lock()

	var req *MemberFetchRequest
	var q *MemberFetchGuildQueue

	// Check to see if this guild is already in the queue
	q, ok := m.notFetching[guildID]
	if !ok {
		q, ok = m.fetching[guildID]
	}

	if ok {
		// The guild's queue already exist
		for _, elem := range q.Queue {
			if elem.Member == userID && (!requireJoinedAt || elem.NeedJoinedAt) {
				// The member is already queued up
				req = elem
				break
			}
		}
	}

	// Request is nil, member was not already requests before
	if req == nil {
		req = &MemberFetchRequest{
			Member:       userID,
			Guild:        guildID,
			NeedJoinedAt: requireJoinedAt,
		}

		if q == nil {
			// Qeueu is nil, this guild does not currently have a queue, create one
			q = &MemberFetchGuildQueue{
				Queue: make([]*MemberFetchRequest, 0, 1),
			}
			m.notFetching[guildID] = q
		}

		q.Queue = append(q.Queue, req)
	}

	// Add the result channel to the request waiting channel
	resultChan := make(chan MemberFetchResult)
	req.WaitingChannels = append(req.WaitingChannels, resultChan)
	m.Unlock()
	return resultChan
}

func (m *memberFetcher) Run() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			m.check()
		case <-m.RunChan:
			m.check()
		}
	}
}

func (m *memberFetcher) check() {
	m.Lock()

	for k, v := range m.notFetching {
		m.fetching[k] = v
		delete(m.notFetching, k)
		go m.runGuild(k)
	}

	m.Unlock()
}

var metricsMemberFetcherRequests = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_memberfetcher_requests_total",
	Help: "The total number of processed events",
})

func (m *memberFetcher) runGuild(guildID int64) {
	for {
		if !m.next(guildID) {
			break
		}
	}
}

func (m *memberFetcher) next(guildID int64) (more bool) {
	m.Lock()

	if len(m.fetching[guildID].Queue) < 1 {
		// Done processing this guild queue
		delete(m.fetching, guildID)
		m.Unlock()
		return false
	}

	q := m.fetching[guildID]
	elem := q.Queue[0]

	m.Unlock()

	logger.WithField("guild", guildID).WithField("user", elem.Member).Debug("Requesting guild member")

	if gs := State.Guild(true, guildID); gs != nil {
		if member := gs.MemberCopy(true, elem.Member); member != nil && member.MemberSet && (!elem.NeedJoinedAt || !member.JoinedAt.IsZero()) {
			// Member is already in state, no need to request it
			m.Lock()

			result := MemberFetchResult{
				Member: member,
			}
			elem.sendResult(result)
			q.Queue = q.Queue[1:]
			m.Unlock()
			return true
		}
	}

	var ms *dstate.MemberState
	var err error
	// Check if this was previously attempted and failed
	failedCacheKey := discordgo.StrID(guildID) + ":" + discordgo.StrID(elem.Member)
	if failedUsersCache.Get(failedCacheKey) == nil {
		metricsMemberFetcherRequests.Inc()

		var member *discordgo.Member
		member, err = common.BotSession.GuildMember(guildID, elem.Member)
		if err == nil && (member != nil && member.User == nil) {
			err = errors.New("User is nil")
		}

		if err != nil {
			logger.WithField("guild", guildID).WithField("user", elem.Member).WithError(err).Debug("Failed fetching member")
			code, _ := common.DiscordError(err)
			if code == discordgo.ErrCodeUnknownUser {
				failedUsersCache.Set(failedCacheKey, 1, time.Hour)
			}
		} else {
			member.GuildID = guildID
			go eventsystem.EmitEvent(eventsystem.NewEventData(nil, eventsystem.EventMemberFetched, &discordgo.GuildMemberAdd{Member: member}), eventsystem.EventMemberFetched)
			if member == nil {
				panic("nil member")
			}

			if member.User == nil {
				panic("nil user")
			}

			if gs := State.Guild(true, guildID); gs != nil {
				gs.MemberAddUpdate(true, member)
				ms = gs.MemberCopy(true, member.User.ID)
			}
		}
	} else {
		err = errors.New("Member is in failed fetching cache")
	}

	if ms == nil && err == nil {
		err = errors.New("Member not found (left guild perhaps?)")
	}

	m.Lock()
	result := MemberFetchResult{
		Err:    err,
		Member: ms,
	}

	elem.sendResult(result)
	q.Queue = q.Queue[1:]

	m.Unlock()
	return true
}

func (m *memberFetcher) Status() (fetching, notFetching int) {
	m.RLock()

	fetching = len(m.fetching)
	notFetching = len(m.notFetching)

	m.RUnlock()

	return
}
