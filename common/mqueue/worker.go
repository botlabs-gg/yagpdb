package mqueue

import (
	"sync"
	"time"

	"github.com/jonas747/yagpdb/bot"
)

const MaxConcurrentSends = 2

type workItem struct {
	elem *QueuedElement
	raw  []byte
}

type workResult struct {
	item  *workItem
	retry bool
}

// MqueueServer is a worker that processes mqueue items for the current shards on the process
// It uses primarily pubsub but it initializes the list by checking the sorted list
type MqueueServer struct {
	PushWork       chan *workItem
	clearTotalWork chan bool
	Stop           chan *sync.WaitGroup

	refreshWork chan bool
	doneWork    chan *workResult

	// Optimisation, we cache the total work even not relevant to the current shards to speed up things like
	// cold starts, otherwise we would have to refetsh the list during each shard start, which for 64
	// shards would be 64 times
	// This is exponential by how big the list currently is
	// (e.g if there's say 100k elements, it would have to transfer around 3GB data for 64 shards)
	// we only cache the list for a short duration a full refresh to avoid it growing out of control
	totalWork        []*workItem
	totalWorkSetAt   time.Time
	totalWorkPresent bool

	// work related to the current shards on this cluster
	localWork []*workItem

	// work currently being processed
	activeWork      []*workItem
	ratelimitedWork int

	backend   Storage
	processor ItemProcessor

	forceAllShards bool
}

func NewServer(backend Storage, processor ItemProcessor) *MqueueServer {
	return &MqueueServer{
		PushWork:       make(chan *workItem),
		clearTotalWork: make(chan bool),
		Stop:           make(chan *sync.WaitGroup),
		refreshWork:    make(chan bool),
		doneWork:       make(chan *workResult),
		backend:        backend,
		processor:      processor,
	}
}

func (m *MqueueServer) Run() {
	for {
		select {
		case wg := <-m.Stop:
			logger.Info("Shutting down mqueue")
			wg.Done()
			return
		case force := <-m.refreshWork:
			m.doRefreshWork(force)
			// For example a new shard started, refresh the localWork slice against totalWork (if present)
		case <-m.clearTotalWork:
			m.totalWork = nil
			m.totalWorkPresent = false
		case wi := <-m.PushWork:
			m.addWork(wi)
		case wi := <-m.doneWork:
			m.finishWork(wi)
		}
	}
}

// performs a full refresh of the local and total work slice
// does not pull the full work slice if totalWorkPresent is set and force is false
func (m *MqueueServer) doRefreshWork(force bool) {
	logger.Infof("Refreshing work, forced: %v", force)
	if force || !m.totalWorkPresent {
		err := m.refreshTotalWork()
		if err != nil {
			return
		}
	}

	m.refreshLocalWorkCached()
}

func (m *MqueueServer) refreshTotalWork() error {
	total, err := m.backend.GetFullQueue()
	if err != nil {
		logger.WithError(err).Error("Failed polling redis mqueue")
		return err
	}

	m.totalWork = total
	m.totalWorkPresent = true
	m.totalWorkSetAt = time.Now()

	time.AfterFunc(time.Minute, func() {
		m.clearTotalWork <- true
	})

	return nil
}

func (m *MqueueServer) refreshLocalWorkCached() {
OUTER:
	for _, wi := range m.totalWork {
		if !bot.ReadyTracker.IsGuildShardReady(wi.elem.GuildID) && !m.forceAllShards {
			continue
		}

		for _, v := range m.localWork {
			if v.elem.ID == wi.elem.ID {
				continue OUTER
			}
		}

		m.localWork = append(m.localWork, wi)
	}

	m.checkRunNextWork()
}

func (m *MqueueServer) addWork(wi *workItem) {
	for _, v := range m.localWork {
		if v.elem.ID == wi.elem.ID {
			return
		}
	}

	m.localWork = append(m.localWork, wi)
	m.checkRunNextWork()
}

func (m *MqueueServer) checkRunNextWork() {
	next := m.findWork()
	if next == nil {
		return
	}

	m.activeWork = append(m.activeWork, next)
	go m.processor.ProcessItem(m.doneWork, next)
}

func (m *MqueueServer) finishWork(wr *workResult) {
	if !wr.retry {
		m.backend.DelItem(wr.item)
		m.activeWork = removeFromWorkSlice(m.activeWork, wr.item)
		m.localWork = removeFromWorkSlice(m.localWork, wr.item)
		if m.totalWorkPresent {
			m.totalWork = removeFromWorkSlice(m.totalWork, wr.item)
		}
	}

	m.checkRunNextWork()
}

func removeFromWorkSlice(s []*workItem, wi *workItem) []*workItem {
	for i, v := range s {
		if v.elem.ID == wi.elem.ID {
			s = append(s[:i], s[i+1:]...)
			return s
		}
	}

	return s
}

func (m *MqueueServer) findWork() *workItem {
	if len(m.activeWork)-m.ratelimitedWork >= MaxConcurrentSends {
		return nil
	}

	// find a work item that does not share a channel with any other item being processed (so ratelimits only take up max 1 worker)
OUTER:
	for _, v := range m.localWork {
		// Don't send 2 messages in a channel at the same time
		for _, active := range m.activeWork {
			if active.elem.ChannelID == v.elem.ChannelID {
				continue OUTER
			}
		}

		// check the ratelimit for this channel, we skip elements being ratelimited
		// var ratelimitWait time.Duration
		// if v.elem.UseWebhook {
		// 	b := webhookSession.Ratelimiter.GetBucket(discordgo.EndpointWebhookToken(v.elem.ChannelID))
		// 	b.Lock()
		// 	ratelimitWait = webhookSession.Ratelimiter.GetWaitTime(b, 1)
		// 	b.Unlock()
		// } else {
		// 	b := common.BotSession.Ratelimiter.GetBucket(discordgo.EndpointChannelMessages(v.elem.ChannelID))
		// 	b.Lock()
		// 	ratelimitWait = common.BotSession.Ratelimiter.GetWaitTime(b, 1)
		// 	b.Unlock()
		// }

		// if ratelimitWait > time.Millisecond*250 {
		// 	continue
		// }

		return v
	}

	return nil
}

type ItemProcessor interface {
	ProcessItem(resp chan *workResult, wi *workItem)
}
