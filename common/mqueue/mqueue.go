package mqueue

import (
	"container/list"
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	sources  = make(map[string]PluginWithErrorHandler)
	stopChan = make(chan *sync.WaitGroup)

	currentlyProcessing     = make([]int64, 0)
	currentlyProcessingLock sync.RWMutex

	startedLock sync.Mutex
	started     bool

	numWorkers = new(int32)
)

type PluginWithErrorHandler interface {
	HandleMQueueError(elem *QueuedElement, err error)
}

var (
	_ bot.LateBotInitHandler = (*Plugin)(nil)
	_ bot.BotStopperHandler  = (*Plugin)(nil)
)

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "mqueue"
}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
}

func RegisterSource(name string, source PluginWithErrorHandler) {
	sources[name] = source
}

func IncrIDCounter() (next int64) {
	err := common.RedisPool.Do(radix.Cmd(&next, "INCR", "mqueue_id_counter"))
	if err != nil {
		logrus.WithError(err).Error("Failed increasing mqueue id counter")
		return -1
	}

	return next
}

func QueueMessageString(source, sourceID string, guildID, channel int64, message string) {
	QueueMessage(source, sourceID, guildID, channel, message, nil)
}

func QueueMessageEmbed(source, sourceID string, guildID, channel int64, embed *discordgo.MessageEmbed) {
	QueueMessage(source, sourceID, guildID, channel, "", embed)
}

func QueueMessage(source, sourceID string, guildID, channel int64, msgStr string, embed *discordgo.MessageEmbed) {
	nextID := IncrIDCounter()
	if nextID == -1 {
		return
	}

	elem := &QueuedElement{
		ID:           nextID,
		Source:       source,
		SourceID:     sourceID,
		Channel:      channel,
		MessageStr:   msgStr,
		MessageEmbed: embed,
		Guild:        guildID,
	}

	serialized, err := json.Marshal(elem)
	if err != nil {
		logrus.WithError(err).Error("Failed marshaling mqueue element")
		return
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZADD", "mqueue", "-1", string(serialized)))
	if err != nil {
		return
	}
}

func (p *Plugin) LateBotInit() {
	go startPolling()
	go processWorker()
	go workerScaler()
}

func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	startedLock.Lock()
	if !started {
		startedLock.Unlock()
		wg.Done()
		return
	}
	startedLock.Unlock()
	stopChan <- wg
}

func workerScaler() {
	var lastWorkerSpawnedAt time.Time
	t := time.NewTicker(time.Second * 10)

	deltaHistory := list.New()
	sizeHistory := list.New()

	lastSize := 0
	for {
		<-t.C

		workmu.Lock()
		current := len(workSlice)
		workmu.Unlock()

		delta := current - lastSize
		lastSize = current
		deltaHistory.PushBack(delta)
		sizeHistory.PushBack(current)

		if deltaHistory.Len() > 6*5 { // keep 5 minute average
			deltaHistory.Remove(deltaHistory.Front())
			sizeHistory.Remove(sizeHistory.Front())
		}

		// see if we should launch a worker
		if current < 100 || time.Since(lastWorkerSpawnedAt) < time.Minute*6 || deltaHistory.Len() < 6 {
			// don't bother launching workers when below 100, and atleast have a minute of averages
			continue
		}

		// calculate average to see if it increased or decreased
		deltaAverage := calcListAverage(deltaHistory)
		sizeAverage := calcListAverage(sizeHistory)

		if deltaAverage > 1 && sizeAverage > 100 {
			logrus.Info("Launched new mqueue worker, total workers: ", atomic.LoadInt32(numWorkers)+1)
			go processWorker()
			lastWorkerSpawnedAt = time.Now()
		}
	}
}

func calcListAverage(in *list.List) int {
	total := 0
	for elem := in.Front(); elem != nil; elem = elem.Next() {
		total += elem.Value.(int)
	}

	average := total / in.Len()
	return average
}

func startPolling() {
	startedLock.Lock()
	if started {
		startedLock.Unlock()
		panic("Already started mqueue")
	}
	started = true
	startedLock.Unlock()

	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case wg := <-stopChan:
			shutdown(wg)
			return
		case <-ticker.C:
			pollRedis()
			if common.Statsd != nil {
				workmu.Lock()
				l := len(workSlice)
				workmu.Unlock()

				common.Statsd.Gauge("yagpdb.mqueue.size", float64(l), []string{"node:" + bot.NodeID()}, 1)
				common.Statsd.Gauge("yagpdb.mqueue.numworkers", float64(atomic.LoadInt32(numWorkers)), []string{"node:" + bot.NodeID()}, 1)
			}
		}
	}
}

func shutdown(wg *sync.WaitGroup) {
	for i := 0; i < 10; i++ {
		currentlyProcessingLock.RLock()
		num := len(currentlyProcessing)
		currentlyProcessingLock.RUnlock()
		if num < 1 {
			break
		}
		time.Sleep(time.Second)
	}
	wg.Done()
}

func pollRedis() {
	var results [][]byte

	// Get all elements that are currently not being processed, we set thhem to processing by setting their score to our run counter, which increases every time the bot starts
	max := strconv.FormatInt(common.CurrentRunCounter, 10)
	err := common.RedisPool.Do(radix.Cmd(&results, "ZRANGEBYSCORE", "mqueue", "-1", "("+max))
	if err != nil {
		logrus.WithError(err).Error("Failed polling redis mqueue")
		return
	}

	if len(results) < 1 {
		return
	}

	// smooth it out over 5 seconds to lower chance of global ratelimits
	sleepPerElem := 5000 / len(results)

	common.RedisPool.Do(radix.WithConn("mqueue", func(rc radix.Conn) error {
		for _, elem := range results {

			var parsed *QueuedElement
			err := json.Unmarshal(elem, &parsed)
			if err != nil {
				logrus.WithError(err).Error("Failed parsing mqueue redis elemtn")
				continue
			}

			if !bot.IsGuildOnCurrentProcess(parsed.Guild) {
				continue
			}

			// Mark it as being processed so it wont get caught in further polling, unless its a new process in which case it wasnt completed
			rc.Do(radix.FlatCmd(nil, "ZADD", "mqueue", common.CurrentRunCounter, string(elem)))

			workmu.Lock()
			workSlice = append(workSlice, &WorkItem{
				elem: parsed,
				raw:  elem,
			})
			workmu.Unlock()

			time.Sleep(time.Duration(sleepPerElem) * time.Millisecond)
		}

		return nil
	}))
}

type WorkItem struct {
	elem *QueuedElement
	raw  []byte
}

var (
	workSlice  []*WorkItem
	activeWork []*WorkItem
	workmu     sync.Mutex
)

func processWorker() {
	atomic.AddInt32(numWorkers, 1)
	defer atomic.AddInt32(numWorkers, -1)

	var currentItem *WorkItem
	for {
		workmu.Lock()

		// if were done processing a item previously then remove it from the active processing slice
		if currentItem != nil {
			for i, v := range activeWork {
				if v == currentItem {
					activeWork = append(activeWork[:i], activeWork[i+1:]...)
					break
				}
			}
			currentItem = nil
		}

		// no new work to process
		if len(workSlice) < 1 {
			workmu.Unlock()
			time.Sleep(time.Second * 3)
			continue
		}

		// find a work item that does not share a channel with any other item being processed (so ratelimits only take up max 1 worker)
		workItemIndex := -1

	OUTER:
		for i, v := range workSlice {
			for _, active := range activeWork {
				if active.elem.Channel == v.elem.Channel {
					continue OUTER
				}
			}

			workItemIndex = i
			break
		}

		// did not find any
		if workItemIndex == -1 {
			workmu.Unlock()
			time.Sleep(time.Second * 3)
			continue
		}

		currentItem = workSlice[workItemIndex]
		activeWork = append(activeWork, currentItem)
		workSlice = append(workSlice[:workItemIndex], workSlice[workItemIndex+1:]...)
		workmu.Unlock()

		process(currentItem.elem, currentItem.raw)
	}
}

func process(elem *QueuedElement, raw []byte) {
	id := elem.ID

	queueLogger := logrus.WithField("mq_id", id)

	defer func() {
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "mqueue", string(raw)))
	}()

	parsedChannel := elem.Channel

	for {
		var err error
		if elem.MessageStr != "" {
			_, err = common.BotSession.ChannelMessageSend(parsedChannel, elem.MessageStr)
		} else if elem.MessageEmbed != nil {
			_, err = common.BotSession.ChannelMessageSendEmbed(parsedChannel, elem.MessageEmbed)
		} else {
			queueLogger.Error("MQueue: Both MessageEmbed and MessageStr empty")
			break
		}

		if err == nil {
			break
		}

		if e, ok := err.(*discordgo.RESTError); ok {
			if (e.Response != nil && e.Response.StatusCode >= 400 && e.Response.StatusCode < 500) || (e.Message != nil && e.Message.Code != 0) {
				if source, ok := sources[elem.Source]; ok {
					source.HandleMQueueError(elem, err)
				}
				break
			}
		}

		queueLogger.Warn("MQueue: Non-discord related error when sending message, retrying. ", err)
		time.Sleep(time.Second)
	}
}
