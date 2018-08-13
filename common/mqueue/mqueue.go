package mqueue

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-kallax.v1"
	"strconv"
	"sync"
	"time"
)

var (
	sources  = make(map[string]PluginWithErrorHandler)
	stopChan = make(chan *sync.WaitGroup)

	currentlyProcessing     = make([]int64, 0)
	currentlyProcessingLock sync.RWMutex

	store *QueuedElementStore

	startedLock sync.Mutex
	started     bool
)

type PluginWithErrorHandler interface {
	HandleMQueueError(elem *QueuedElementNoKallax, err error)
}

var (
	_ bot.BotInitHandler    = (*Plugin)(nil)
	_ bot.BotStopperHandler = (*Plugin)(nil)
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

func InitStores() {
	// Init table
	_, err := common.PQ.Exec(`CREATE TABLE IF NOT EXISTS mqueue (
	id serial NOT NULL PRIMARY KEY,
	source text NOT NULL,
	source_id text NOT NULL,
	message_str text NOT NULL,
	message_embed text NOT NULL,
	channel text NOT NULL,
	processed boolean NOT NULL
);

CREATE INDEX IF NOT EXISTS mqueue_processed_x ON mqueue(processed);
`)
	if err != nil {
		panic("mqueue: " + err.Error())
	}

	store = NewQueuedElementStore(common.PQ)
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

func QueueMessageString(source, sourceID, channel, message string) {
	QueueMessage(source, sourceID, channel, message, nil)
}

func QueueMessageEmbed(source, sourceID, channel string, embed *discordgo.MessageEmbed) {
	QueueMessage(source, sourceID, channel, "", embed)
}

func QueueMessage(source, sourceID, channel string, msgStr string, embed *discordgo.MessageEmbed) {
	nextID := IncrIDCounter()
	if nextID == -1 {
		return
	}

	elem := &QueuedElementNoKallax{
		ID:           nextID,
		Source:       source,
		SourceID:     sourceID,
		Channel:      channel,
		MessageStr:   msgStr,
		MessageEmbed: embed,
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

func (p *Plugin) BotInit() {
	go startPolling()
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

func startPolling() {
	startedLock.Lock()
	if started {
		startedLock.Unlock()
		panic("Already started mqueue")
	}
	started = true
	startedLock.Unlock()

	ticker := time.NewTicker(time.Second)
	tickerClean := time.NewTicker(time.Hour)
	for {
		select {
		case wg := <-stopChan:
			shutdown(wg)
			return
		case <-ticker.C:
			pollLegacy()
			pollRedis()
		case <-tickerClean.C:
			go func() {
				result, err := common.PQ.Exec("DELETE FROM mqueue WHERE processed=true")
				if err != nil {
					logrus.WithError(err).Error("Failed cleaning mqueue db")
				} else {
					rows, err := result.RowsAffected()
					if err == nil {
						logrus.Println("mqueue cleaned up ", rows, " rows")
					}
				}
			}()
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

func pollLegacy() {

	// Grab the current processing elements
	currentlyProcessingLock.RLock()
	processing := make([]int64, len(currentlyProcessing))
	copy(processing, currentlyProcessing)
	currentlyProcessingLock.RUnlock()

	elems, err := store.FindAll(NewQueuedElementQuery().Where(kallax.Eq(Schema.QueuedElement.Processed, false)))
	if err != nil {
		logrus.WithError(err).Error("MQueue: Failed polling message queue")
		return
	}

	currentlyProcessingLock.Lock()
OUTER:
	for _, v := range elems {
		for _, current := range processing {
			if v.ID == current {
				continue OUTER
			}
		}
		currentlyProcessing = append(currentlyProcessing, v.ID)
		go process(v, nil, nil, true)
		logrus.Info("Legacy handling element")
	}
	currentlyProcessingLock.Unlock()
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

	logrus.Println("Got ", len(results), " results")

	common.RedisPool.Do(radix.WithConn("mqueue", func(rc radix.Conn) error {
		for _, elem := range results {
			// Mark it as being processed so it wont get caught in further polling, unless its a new process in which case it wasnt completed
			rc.Do(radix.FlatCmd(nil, "ZADD", "mqueue", common.CurrentRunCounter, string(elem)))

			var parsed *QueuedElementNoKallax
			err := json.Unmarshal(elem, &parsed)
			if err != nil {
				logrus.WithError(err).Error("Failed parsing mqueue redis elemtn")
				continue
			}

			go process(nil, parsed, elem, false)
			logrus.Println("New handling element")
		}

		return nil
	}))
}

func process(elem *QueuedElement, elemSimple *QueuedElementNoKallax, elemSimpleRaw []byte, isLegacy bool) {
	id := int64(0)
	if elem != nil {
		id = elem.ID
	} else {
		id = elemSimple.ID
	}

	queueLogger := logrus.WithField("mq_id", id).WithField("legacy", isLegacy)

	defer func() {
		if !isLegacy {
			common.RedisPool.Do(radix.Cmd(nil, "ZREM", string(elemSimpleRaw)))
			return
		}

		elem.Processed = true
		_, err := store.Save(elem)
		if err != nil {
			queueLogger.WithError(err).Error("MQueue: Failed marking elem as processed")
		}

		currentlyProcessingLock.Lock()
		for i, v := range currentlyProcessing {
			if v == elem.ID {
				currentlyProcessing = append(currentlyProcessing[:i], currentlyProcessing[i+1:]...)
				break
			}
		}
		currentlyProcessingLock.Unlock()
	}()

	if elemSimple == nil {
		elemSimple = NewElemFromKallax(elem)
	}

	parsedChannel, err := strconv.ParseInt(elemSimple.Channel, 10, 64)
	if err != nil {
		queueLogger.WithError(err).Error("Failed parsing Channel")
	}

	for {
		var err error
		if elemSimple.MessageStr != "" {
			_, err = common.BotSession.ChannelMessageSend(parsedChannel, elemSimple.MessageStr)
		} else if elemSimple.MessageEmbed != nil {
			_, err = common.BotSession.ChannelMessageSendEmbed(parsedChannel, elemSimple.MessageEmbed)
		} else {
			queueLogger.Error("MQueue: Both MessageEmbed and MessageStr empty")
			break
		}

		if err == nil {
			break
		}

		if e, ok := err.(*discordgo.RESTError); ok {
			if (e.Response != nil && e.Response.StatusCode >= 400 && e.Response.StatusCode < 500) || (e.Message != nil && e.Message.Code != 0) {
				if source, ok := sources[elemSimple.Source]; ok {
					source.HandleMQueueError(elemSimple, err)
				}
				break
			}
		}

		queueLogger.Warn("MQueue: Non-discord related error when sending message, retrying. ", err)
		time.Sleep(time.Second)
	}
}
