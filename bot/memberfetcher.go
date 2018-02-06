package bot

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"sync"
	"time"
)

var (
	MemberFetcher = &memberFetcher{
		fetching:    make(map[string]*MemberFetchGuildQueue),
		notFetching: make(map[string]*MemberFetchGuildQueue),
	}
)

func GetMember(guildID, userID string) (*discordgo.Member, error) {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	cop := gs.MemberCopy(true, userID, true)
	if cop != nil {
		return cop, nil
	}

	result := <-MemberFetcher.RequestMember(guildID, userID)
	return result.Member, result.Err
}

func GetMembers(guildID string, userIDs ...string) ([]*discordgo.Member, error) {
	resultChan := make(chan *discordgo.Member)
	for _, v := range userIDs {
		go func(id string) {
			m, _ := GetMember(guildID, id)
			resultChan <- m
		}(v)
	}

	result := make([]*discordgo.Member, 0, len(userIDs))
	for i := 0; i < len(userIDs); i++ {
		m := <-resultChan
		if m != nil {
			result = append(result, m)
		}
	}

	return result, nil
}

// memberFetcher handles a per guild queue for fetching members
// This is probably overkill as the root cause for the flood of member requests (state being flushed upon guilds becoming unavailble) was fixed
// But it's here, for better or for worse
type memberFetcher struct {
	sync.RWMutex

	// Queue of guilds to user id's to fetch
	fetching    map[string]*MemberFetchGuildQueue
	notFetching map[string]*MemberFetchGuildQueue

	// Signal to run immediately
	RunChan chan bool
	Stop    chan bool
}

type MemberFetchGuildQueue struct {
	Queue []*MemberFetchRequest
}

type MemberFetchRequest struct {
	Member          string
	Guild           string
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
	Member *discordgo.Member
}

func (m *memberFetcher) RequestMember(guildID, userID string) <-chan MemberFetchResult {
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
			if elem.Member == userID {
				// The member is already queued up
				req = elem
				break
			}
		}
	}

	// Request is nil, member was not already requests before
	if req == nil {
		req = &MemberFetchRequest{
			Member: userID,
			Guild:  guildID,
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

func (m *memberFetcher) runGuild(guildID string) {
	for {
		if !m.next(guildID) {
			break
		}
	}
}

func (m *memberFetcher) next(guildID string) (more bool) {
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

	logrus.WithField("guild", guildID).WithField("user", elem.Member).Debug("Requesting guild member")

	if gs := State.Guild(true, guildID); gs != nil {
		if member := gs.MemberCopy(true, elem.Member, true); member != nil {
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

	member, err := common.BotSession.GuildMember(guildID, elem.Member)
	if err != nil {
		logrus.WithField("guild", guildID).WithField("user", elem.Member).WithError(err).Debug("Failed fetching member")
	} else {
		go eventsystem.EmitEvent(&eventsystem.EventData{
			EventDataContainer: &eventsystem.EventDataContainer{
				GuildMemberAdd: &discordgo.GuildMemberAdd{Member: member},
			},
			Type: eventsystem.EventMemberFetched,
		}, eventsystem.EventMemberFetched)

		if gs := State.Guild(true, guildID); gs != nil {
			gs.MemberAddUpdate(true, member)
		}
	}

	m.Lock()
	result := MemberFetchResult{
		Err:    err,
		Member: member,
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
