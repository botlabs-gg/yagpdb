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
		fetching:    make(map[string][]*MemberFetchRequest),
		notFetching: make(map[string][]*MemberFetchRequest),
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

type memberFetcher struct {
	sync.RWMutex

	// Queue of guilds to user id's to fetch
	fetching    map[string][]*MemberFetchRequest
	notFetching map[string][]*MemberFetchRequest

	// Signal to run immediately
	RunChan chan bool
	Stop    chan bool
}

type MemberFetchRequest struct {
	Member          string
	Guild           string
	WaitingChannels []chan MemberFetchResult
}

func (req *MemberFetchRequest) sendResult(result MemberFetchResult) {
	for _, ch := range req.WaitingChannels {
		go func() {
			ch <- result
		}()
	}
}

type MemberFetchResult struct {
	Err    error
	Member *discordgo.Member
}

func (m *memberFetcher) RequestMember(guildID, userID string) <-chan MemberFetchResult {
	m.Lock()

	var req *MemberFetchRequest
	var q []*MemberFetchRequest

	// Check to see if this member is already requested
	q, ok := m.notFetching[guildID]
	if !ok {
		q, ok = m.fetching[guildID]
	}

	if ok {
		for _, elem := range q {
			if elem.Member == userID {
				req = elem
			}
		}
	}

	// Not requested already, queue it up
	if req == nil {
		req = &MemberFetchRequest{
			Member: userID,
			Guild:  guildID,
		}
		m.notFetching[guildID] = append(m.notFetching[guildID], req)
	}

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

	if len(m.fetching[guildID]) < 1 {
		// Done processing this guild queue
		delete(m.fetching, guildID)
		m.Unlock()
		return false
	}

	elem := m.fetching[guildID][0]

	m.Unlock()

	logrus.WithField("guild", guildID).WithField("user", elem.Member).Info("Requesting guild member")

	if gs := State.Guild(true, guildID); gs != nil {
		if member := gs.MemberCopy(true, elem.Member, true); member != nil {
			// Member is already in state, no need to request it
			m.Lock()

			result := MemberFetchResult{
				Member: member,
			}
			elem.sendResult(result)
			m.fetching[guildID] = m.fetching[guildID][1:]
			m.Unlock()
			return true
		}
	}

	member, err := common.BotSession.GuildMember(guildID, elem.Member)
	if err != nil {
		logrus.WithField("guild", guildID).WithField("user", elem.Member).WithError(err).Error("Failed fetching member")
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
	m.fetching[guildID] = m.fetching[guildID][1:]
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
