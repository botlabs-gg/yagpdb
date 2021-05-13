package mqueue

import (
	"encoding/json"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
)

const MaxConcurrentSends = 2

type workResult struct {
	item  *workItem
	retry bool
}

type MqueueServer struct {
	refreshWork    chan bool
	doneWork       chan *workResult
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
	activeWork      []*workItem
	ratelimitedWork int
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

func (m *MqueueServer) checkRunNextWork() {
	next := m.findWork()
	if next == nil {
		return
	}

	m.activeWork = append(m.activeWork, next)
	go m.runWork(next)
}

func (m *MqueueServer) runWork(wi *workItem) {
	metricsProcessed.With(prometheus.Labels{"source": wi.elem.Source}).Inc()

	retry := false
	defer func() {
		m.doneWork <- &workResult{
			item:  wi,
			retry: retry,
		}
	}()

	queueLogger := logger.WithField("mq_id", wi.elem.ID)

	defer func() {
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "mqueue", string(wi.raw)))
	}()

	var err error
	if wi.elem.UseWebhook {
		err = trySendWebhook(queueLogger, wi.elem)
	} else {
		err = trySendNormal(queueLogger, wi.elem)
	}

	if err == nil {
		return
	}

	if e, ok := errors.Cause(err).(*discordgo.RESTError); ok {
		if (e.Response != nil && e.Response.StatusCode >= 400 && e.Response.StatusCode < 500) || (e.Message != nil && e.Message.Code != 0) {
			if source, ok := sources[wi.elem.Source]; ok {
				maybeDisableFeed(source, wi.elem, e)
			}

			return
		}
	} else {
		if onGuild, err := common.BotIsOnGuild(wi.elem.Guild); !onGuild && err == nil {
			if source, ok := sources[wi.elem.Source]; ok {
				logger.WithError(err).Warnf("disabling feed item %s from %s to nonexistant guild", wi.elem.SourceID, wi.elem.Source)
				source.DisableFeed(wi.elem, err)
			}

			return
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}
	}

	if c, _ := common.DiscordError(err); c != 0 {
		return
	}

	retry = true
	queueLogger.Warn("Non-discord related error when sending message, retrying. ", err)
	time.Sleep(time.Second)

}

func (m *MqueueServer) finishWork(wr *workResult) {
	if !wr.retry {
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "mqueue", string(wr.item.raw)))
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

		// if ratelimitWait > time.Millisecond*250 {
		// 	continue
		// }

		return v
	}

	return nil
}
