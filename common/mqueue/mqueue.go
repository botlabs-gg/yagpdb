package mqueue

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"strconv"
	"sync"
	"time"
)

var (
	sources  = make(map[string]PluginWithErrorHandler)
	stopChan = make(chan *sync.WaitGroup)

	currentlyProcessing     = make([]int64, 0)
	currentlyProcessingLock sync.RWMutex

	startedLock sync.Mutex
	started     bool
)

type PluginWithErrorHandler interface {
	HandleMQueueError(elem *QueuedElement, err error)
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

func QueueMessageString(source, sourceID string, guildID, channelID int64, message string) {
	QueueMessage(source, sourceID, guildID, channelID, message, nil)
}

func QueueMessageEmbed(source, sourceID string, guildID, channelID int64, embed *discordgo.MessageEmbed) {
	QueueMessage(source, sourceID, guildID, channelID, "", embed)
}

func QueueMessage(source, sourceID string, guildID, channelID int64, msgStr string, embed *discordgo.MessageEmbed) {
	nextID := IncrIDCounter()
	if nextID == -1 {
		return
	}

	elem := &QueuedElement{
		ID:           nextID,
		Source:       source,
		SourceID:     sourceID,
		Channel:      channelID,
		Guild:        guildID,
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

func pollRedis() {
	var results [][]byte

	// Get all elements that are currently not being processed, we set thhem to processing by setting their score to our run counter, which increases every time the bot starts
	max := strconv.FormatInt(common.CurrentRunCounter, 10)
	err := common.RedisPool.Do(radix.Cmd(&results, "ZRANGEBYSCORE", "mqueue", "-1", "("+max))
	if err != nil {
		logrus.WithError(err).Error("Failed polling redis mqueue")
		return
	}

	common.RedisPool.Do(radix.WithConn("mqueue", func(rc radix.Conn) error {
		for _, elem := range results {
			// Mark it as being processed so it wont get caught in further polling, unless its a new process in which case it wasnt completed
			rc.Do(radix.FlatCmd(nil, "ZADD", "mqueue", common.CurrentRunCounter, string(elem)))

			var parsed *QueuedElement
			err := json.Unmarshal(elem, &parsed)
			if err != nil {
				logrus.WithError(err).Error("Failed parsing mqueue redis elemtn")
				continue
			}

			go process(parsed, elem)
		}

		return nil
	}))
}

func process(elem *QueuedElement, elemRaw []byte) {
	id := elem.ID

	queueLogger := logrus.WithField("mq_id", id)

	defer func() {
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "mqueue", string(elemRaw)))
	}()

	for {
		var err error
		if elem.MessageStr != "" {
			_, err = common.BotSession.ChannelMessageSend(elem.Channel, elem.MessageStr)
		} else if elem.MessageEmbed != nil {
			_, err = common.BotSession.ChannelMessageSendEmbed(elem.Channel, elem.MessageEmbed)
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
