package bot

import (
	"errors"
	"strconv"
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

	numHandled     int // number of chunks handled
	lastHandledEvt time.Time
	nonce          string
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
			timeout := time.Minute
			if inactiveFor > timeout {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				logger.Errorf("batchMemberManager: job timed out %d, handled: [%d]", v.GuildID, v.numHandled)
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

	job := &batchMemberJob{
		CreatedAt:      time.Now(),
		GuildID:        guildID,
		F:              f,
		lastHandledEvt: time.Now(),
		nonce:          strconv.Itoa(GenNonce()),
	}

	err := m.queueJob(job)
	if err != nil {
		return err
	}

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return errors.New("No session?")
	}
	q := ""
	session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
		GuildID: gs.ID,
		Nonce:   job.nonce,
		Query:   &q,
	})

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
		nonce:          strconv.Itoa(GenNonce()),
	}

	err := m.queueJob(job)
	if err != nil {
		return nil, err
	}

	session := ShardManager.SessionForGuild(guildID)
	if session == nil {
		return nil, errors.New("No session?")
	}

	session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
		GuildID: gs.ID,
		Limit:   1000,
		Query:   &query,
		Nonce:   job.nonce,
	})
	return m.waitResponse(time.Second*10, retCh)
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
			if v.nonce != "" && chunk.Nonce != v.nonce {
				// validate the nonce if set
				continue
			}

			go v.F(chunk.GuildID, chunk.Members)

			v.numHandled++
			v.lastHandledEvt = time.Now()

			if v.numHandled >= chunk.ChunkCount {
				// finished, remove it from active jobs
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			}

			break
		}
	}
}

var (
	nonceOnce sync.Once
	nonceChan chan int
)

func GenNonce() int {
	nonceOnce.Do(func() {
		nonceChan = make(chan int, 10)
		go func() {
			i := 1
			for {
				nonceChan <- i
				i++
			}
		}()
	})

	return <-nonceChan
}
