package mqueue

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"gopkg.in/src-d/go-kallax.v1"
	"sync"
	"time"
)

type PluginWithErrorHandler interface {
	HandleMQueueError(elem *QueuedElement, err error)
}

var (
	sources  = make(map[string]PluginWithErrorHandler)
	stopChan = make(chan *sync.WaitGroup)

	currentlyProcessing     = make([]int64, 0)
	currentlyProcessingLock sync.RWMutex

	store *QueuedElementStore

	startedLock sync.Mutex
	started     bool
)

func InitStores() {
	common.PQ.Exec(`CREATE TABLE IF NOT EXISTS mqueue (
	id serial NOT NULL PRIMARY KEY,
	source text NOT NULL,
	source_id text NOT NULL,
	message_str text NOT NULL,
	message_embed text NOT NULL,
	channel text NOT NULL,
	processed boolean NOT NULL
);`)

	store = NewQueuedElementStore(common.PQ)
}

func RegisterSource(name string, source PluginWithErrorHandler) {
	sources[name] = source
}

func QueueMessageString(source, sourceID, channel, message string) {
	elem := &QueuedElement{
		Source:     source,
		SourceID:   sourceID,
		Channel:    channel,
		MessageStr: message,
	}

	store.Insert(elem)
}

func QueueMessageEmbed(source, sourceID, channel string, embed *discordgo.MessageEmbed) {
	encoded, err := json.Marshal(embed)
	if err != nil {
		logrus.WithError(err).Error("MQueue: Failed encoding message")
		return
	}

	elem := &QueuedElement{
		Source:       source,
		SourceID:     sourceID,
		Channel:      channel,
		MessageEmbed: string(encoded),
	}

	store.Insert(elem)
}

func StartPolling() {
	startedLock.Lock()
	if started {
		startedLock.Unlock()
		panic("Already started mqueue")
	}
	started = true
	startedLock.Unlock()

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case wg := <-stopChan:
			shutdown(wg)
			return
		case <-ticker.C:
			poll()
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

func Stop(wg *sync.WaitGroup) {
	startedLock.Lock()
	if !started {
		startedLock.Unlock()
		return
	}
	startedLock.Unlock()
	wg.Add(1)
	stopChan <- wg
}

func poll() {

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
		go process(v)
	}
	currentlyProcessingLock.Unlock()
}

func process(elem *QueuedElement) {
	queueLogger := logrus.WithField("mq_id", elem.ID)

	defer func() {
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

	var embed *discordgo.MessageEmbed
	if len(elem.MessageEmbed) > 0 {
		err := json.Unmarshal([]byte(elem.MessageEmbed), &embed)
		if err != nil {
			queueLogger.Error("MQueue: Failed decoding message embed")
		}
	}

	for {
		var err error
		if elem.MessageStr != "" {
			_, err = common.BotSession.ChannelMessageSend(elem.Channel, elem.MessageStr)
		} else if embed != nil {
			_, err = common.BotSession.ChannelMessageSendEmbed(elem.Channel, embed)
		} else {
			queueLogger.Error("MQueue: Both MessageEmbed and MessageStr empty")
			break
		}

		if err == nil {
			break
		}

		if e, ok := err.(*discordgo.RESTError); ok && e.Message != nil && e.Message.Code != 0 {
			if source, ok := sources[elem.Source]; ok {
				source.HandleMQueueError(elem, err)
			}
			break
		}

		queueLogger.Warn("MQueue: Non-discord related error when sending message, retrying. ", err)
		time.Sleep(time.Second)
	}
}
