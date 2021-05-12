package mqueue

import (
	"encoding/json"
	"time"

	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
)

const MaxConcurrentSends = 2

type MqueueServer struct {
	refreshWork    chan bool
	doneWork       chan *workItem
	pubsubWork     chan *workItem
	clearTotalWork chan bool

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
	activeWork []*workItem
}

func (m *MqueueServer) Run() {
	go m.runPubsub()

	select {
	case <-m.refreshWork:
		m.doRefreshWork()
		// For example a new shard started, refresh the localWork slice against totalWork (is present)
	case <-m.clearTotalWork:
		m.totalWork = nil
		m.totalWorkPresent = false
	case wi := <-m.pubsubWork:
		m.addWork(wi)
	case wi := <-m.doneWork:
		m.finishWork(wi)
	}
}

func (m *MqueueServer) runPubsub() {
	conn, err := radix.PersistentPubSubWithOpts("tcp", common.ConfRedis.GetString())
	if err != nil {
		panic(err)
	}

	msgChan := make(chan radix.PubSubMessage, 100)
	if err := conn.Subscribe(msgChan, "mqueue_pubsub"); err != nil {
		panic(err)
	}

	for msg := range msgChan {
		if len(msg.Message) < 1 {
			continue
		}

		var dec *QueuedElement
		err = json.Unmarshal(msg.Message, &dec)
		if err != nil {
			logger.WithError(err).Error("failed decoding mqueue pubsub message")
			continue
		}

		m.pubsubWork <- &workItem{
			elem: dec,
			raw:  msg.Message,
		}
	}
}

// performs a full refresh of the local and total work slice
// does not pull the full work slice if totalWorkPresent is set
func (m *MqueueServer) doRefreshWork() {
	if !m.totalWorkPresent {
		err := m.refreshTotalWork()
		if err != nil {
			return
		}
	}

	m.refreshWorkCached()
}

func (m *MqueueServer) refreshTotalWork() error {
	var results [][]byte

	err := common.RedisPool.Do(radix.Cmd(&results, "ZRANGEBYSCORE", "mqueue", "-1", "+inf"))
	if err != nil {
		logger.WithError(err).Error("Failed polling redis mqueue")
		return err
	}

	m.totalWork = make([]*workItem, 0, len(results))

	for _, v := range results {
		var dec QueuedElement
		err = json.Unmarshal(v, &dec)
		if err != nil {
			logger.WithError(err).Error("Failed decoding queued mqueue element from ful refresh")
		} else {
			m.totalWork = append(m.totalWork, &workItem{
				elem: &dec,
				raw:  v,
			})
		}
	}

	m.totalWorkPresent = true
	m.totalWorkSetAt = time.Now()

	time.AfterFunc(time.Minute, func() {
		m.clearTotalWork <- true
	})

	return nil
}

func (m *MqueueServer) refreshWorkCached() {
OUTER:
	for _, wi := range m.totalWork {
		if !bot.ReadyTracker.IsGuildShardReady(wi.elem.Guild) {
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
			continue
		}
	}

	m.localWork = append(m.localWork, wi)
	m.checkRunNextWork()
}

func (m *MqueueServer) checkRunNextWork()       {}
func (m *MqueueServer) finishWork(wi *workItem) {}

func (m *MqueueServer) findWork() *workItem {
	if len(m.activeWork) >= MaxConcurrentSends {
		return nil
	}

	// find a work item that does not share a channel with any other item being processed (so ratelimits only take up max 1 worker)
OUTER:
	for _, v := range m.localWork {
		// Don't send 2 messages in a channel at the same time
		for _, active := range m.activeWork {
			if active.elem.Channel == v.elem.Channel {
				continue OUTER
			}
		}

		// check the ratelimit for this channel, we skip elements being ratelimited
		// var ratelimitWait time.Duration
		// if v.elem.UseWebhook {
		// 	b := webhookSession.Ratelimiter.GetBucket(discordgo.EndpointWebhookToken(v.elem.Channel))
		// 	b.Lock()
		// 	ratelimitWait = webhookSession.Ratelimiter.GetWaitTime(b, 1)
		// 	b.Unlock()
		// } else {
		// 	b := common.BotSession.Ratelimiter.GetBucket(discordgo.EndpointChannelMessages(v.elem.Channel))
		// 	b.Lock()
		// 	ratelimitWait = common.BotSession.Ratelimiter.GetWaitTime(b, 1)
		// 	b.Unlock()
		// }

		if ratelimitWait > time.Millisecond*250 {
			continue
		}

		return v
	}

	return nil
}
