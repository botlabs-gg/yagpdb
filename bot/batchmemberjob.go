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
			if (v.numHandled > 0 && inactiveFor > time.Second*10) ||
				(v.numHandled == 0 && inactiveFor > time.Second*60) ||
				v.numHandled == v.targetCount {
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

	m.mu.Lock()
	m.jobs = append(m.jobs, job)
	m.mu.Unlock()

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return errors.New("No session?")
	}

	session.GatewayManager.RequestGuildMembers(guildID, "", 0)
	return nil
}

func (m *batchMemberJobManager) handleGuildMemberChunk(evt *eventsystem.EventData) {
	chunk := evt.GuildMembersChunk()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range m.jobs {
		if v.GuildID == chunk.GuildID {
			go v.F(chunk.GuildID, chunk.Members)
			v.numHandled += len(chunk.Members)
			v.lastHandledEvt = time.Now()
		}
	}
}
