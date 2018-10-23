package scheduledevents2

//go:generate sqlboiler psql

import (
	"context"
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

type ScheduledEvents struct {
	stop chan *sync.WaitGroup

	currentlyProcessingMU sync.Mutex
	currentlyProcessing   map[int64]bool
}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Fatal("scheduledevents failed initializing db schema")
	}

	common.RegisterPlugin(&ScheduledEvents{
		stop:                make(chan *sync.WaitGroup),
		currentlyProcessing: make(map[int64]bool),
	})
}

func (se *ScheduledEvents) Name() string {
	return "scheduled_events"
}

type HandlerFunc func(evt *models.ScheduledEvent, data interface{}) (err error, retry bool)

type RegisteredHandler struct {
	EvtName    string
	DataFormat interface{}
	Handler    HandlerFunc
}

var (
	registeredHandlers = make(map[string]*RegisteredHandler)
)

func RegisterHandler(eventName string, dataDest interface{}, handler HandlerFunc) {
	registeredHandlers[eventName] = &RegisteredHandler{
		EvtName:    eventName,
		DataFormat: dataDest,
		Handler:    handler,
	}

	logrus.Debug("[ScheduledEvents2] Registered handler for ", eventName)
}

var _ bot.BotStartedHandler = (*ScheduledEvents)(nil)
var _ bot.BotStopperHandler = (*ScheduledEvents)(nil)

func (se *ScheduledEvents) BotStarted() {
	go se.runCheckLoop()
}

func (se *ScheduledEvents) StopBot(wg *sync.WaitGroup) {
	se.stop <- wg
}

func (se *ScheduledEvents) runCheckLoop() {
	t := time.NewTicker(time.Second)
	for {
		select {
		case wg := <-se.stop:
			wg.Done()
			return
		case <-t.C:
			se.check()
		}
	}
}

func (se *ScheduledEvents) check() {
	se.currentlyProcessingMU.Lock()
	defer se.currentlyProcessingMU.Unlock()

	toProcess, err := models.ScheduledEvents(qm.Where("triggers_at < ? AND processed=false", time.Now())).AllG(context.Background())
	if err != nil {
		logrus.WithError(err).Error("[scheduledevents2] failed checking for events to process")
		return
	}

	for _, p := range toProcess {
		if !bot.IsGuildOnCurrentProcess(p.GuildID) {
			continue
		}

		if v := se.currentlyProcessing[p.ID]; v {
			continue
		}

		se.currentlyProcessing[p.ID] = true
		go se.processItem(p)
	}
}

func (se *ScheduledEvents) processItem(item *models.ScheduledEvent) {
	l := logrus.WithField("id", item.ID).WithField("evt", item.EventName)

	handler, ok := registeredHandlers[item.EventName]
	if !ok {
		l.Error("[scheduledevents] unknown event: ", item.EventName)
		se.markDone(item)
		return
	}

	var decodedData interface{}
	if handler.DataFormat != nil {
		typ := reflect.TypeOf(handler.DataFormat)

		// Decode the form into the destination struct
		decodedData = reflect.New(typ).Interface()
		err := json.Unmarshal(item.Data, decodedData)
		if err != nil {
			l.WithError(err).Error("[scheduledevents2] failed decoding event data")
			se.markDone(item)
			return
		}
	}

	for {
		err, retry := handler.Handler(item, decodedData)
		if err != nil {
			l.WithError(err).Error("[scheduledevents2] handler returned an error")
		}

		if retry {
			l.WithError(err).Warn("[scheduledevents2] retrying handling event")
			time.Sleep(time.Second * time.Duration(rand.Intn(10)+5))
			continue
		}

		break
	}
}

func (se *ScheduledEvents) markDone(item *models.ScheduledEvent) {
	item.Processed = true
	_, err := item.UpdateG(context.Background(), boil.Whitelist("processed"))

	se.currentlyProcessingMU.Lock()
	delete(se.currentlyProcessing, item.ID)
	se.currentlyProcessingMU.Unlock()

	if err != nil {
		logrus.WithError(err).Error("[scheduledevents2] failed marking item as processed")
	}
}

func CheckDiscordErrRetry(err error) bool {
	if cast, ok := err.(*discordgo.RESTError); ok {
		if cast.Message != nil && cast.Message.Code != 0 {
			// proper discord response, don't retry
			return false
		}
	}

	// an unknown error unrelated to the discord api occured (503's for example) attempt a retry
	return true
}
