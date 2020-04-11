package bot

import (
	"errors"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
)

var (
	BatchMemberJobManager = newBatchMemberJobManager()
)

type batchMemberJob struct {
	CreatedAt time.Time
	GuildID   int64
	F         func(guildID int64, members []*discordgo.Member)

	targetCount    int
	numHandled     int
	lastHandledEvt time.Time
}

type batchMemberJobManager struct {
	jobs []*batchMemberJob
	mu   sync.Mutex
}

func newBatchMemberJobManager() *batchMemberJobManager {
	m := &batchMemberJobManager{}
	go m.monitor()
	return m
}

func (m *batchMemberJobManager) monitor() {
	ticker := time.NewTicker(time.Second)
	for {
		<-ticker.C
		m.checkall()
	}
}

func (m *batchMemberJobManager) checkall() {
	m.mu.Lock()
	defer m.mu.Unlock()

OUTER:
	for {
		for i, v := range m.jobs {
			inactiveFor := time.Since(v.lastHandledEvt)

			timeout := time.Second * 20
			if v.numHandled > 0 || v.targetCount == -1 {
				timeout = time.Second * 5
			}

			if inactiveFor > timeout || v.numHandled >= v.targetCount {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				logger.Infof("batchMemberManager: Removed expired job on %d, handled: [%d/%d]", v.GuildID, v.numHandled, v.targetCount)
				continue OUTER
			}
		}

		break
	}
}

var (
	ErrGuildNotOnProcess = errors.New("Guild not on process")
)

func (m *batchMemberJobManager) NewBatchMemberJob(guildID int64, f func(guildID int64, member []*discordgo.Member)) error {
	if !ReadyTracker.IsGuildShardReady(guildID) {
		return ErrGuildNotOnProcess
	}

	gs := State.Guild(true, guildID)
	if gs == nil {
		return ErrGuildNotFound
	}

	gs.RLock()
	targetCount := gs.Guild.MemberCount
	gs.RUnlock()

	job := &batchMemberJob{
		CreatedAt:      time.Now(),
		GuildID:        guildID,
		F:              f,
		lastHandledEvt: time.Now(),
		targetCount:    targetCount,
	}

	err := m.queueJob(job)
	if err != nil {
		return err
	}

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return errors.New("No session?")
	}

	session.GatewayManager.RequestGuildMembers(guildID, "", 0)
	return nil
}

func (m *batchMemberJobManager) SearchByUsername(guildID int64, query string) ([]*discordgo.Member, error) {
	if !ReadyTracker.IsGuildShardReady(guildID) {
		return nil, ErrGuildNotOnProcess
	}

	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil, ErrGuildNotFound
	}

	retCh := make(chan []*discordgo.Member)

	job := &batchMemberJob{
		CreatedAt: time.Now(),
		GuildID:   guildID,
		F: func(guildID int64, members []*discordgo.Member) {
			retCh <- members
		},
		lastHandledEvt: time.Now(),
		targetCount:    -1,
	}

	err := m.queueJob(job)
	if err != nil {
		return nil, err
	}

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return nil, errors.New("No session?")
	}

	session.GatewayManager.RequestGuildMembers(guildID, query, 0)
	return m.waitResponse(time.Second*3, retCh)
}

var ErrTimeoutWaitingForMember = errors.New("Timeout waiting for members")

func (m *batchMemberJobManager) waitResponse(timeout time.Duration, retCh chan []*discordgo.Member) ([]*discordgo.Member, error) {
	select {
	case <-time.After(timeout):
		return nil, ErrTimeoutWaitingForMember
	case result := <-retCh:
		return result, nil
	}
}

func (m *batchMemberJobManager) queueJob(job *batchMemberJob) error {
OUTER:
	for {
		m.mu.Lock()
		for _, v := range m.jobs {
			if v.GuildID == job.GuildID {
				// wait until the previous job on this guild is done
				m.mu.Unlock()
				time.Sleep(time.Millisecond * 100)
				continue OUTER
			}
		}

		job.CreatedAt = time.Now()
		job.lastHandledEvt = time.Now()
		m.jobs = append(m.jobs, job)
		m.mu.Unlock()
		return nil
	}
}

func (m *batchMemberJobManager) handleGuildMemberChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, v := range m.jobs {
		if v.GuildID == chunk.GuildID {
			go v.F(chunk.GuildID, chunk.Members)

			v.numHandled += len(chunk.Members)
			v.lastHandledEvt = time.Now()

			if v.numHandled >= v.targetCount {
				// remove it from active jobs
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			}

			break
		}
	}
}
