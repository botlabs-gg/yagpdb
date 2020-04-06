package scheduledevents2

//go:generate sqlboiler psql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/volatiletech/null"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/volatiletech/sqlboiler/boil"
)

type ScheduledEvents struct {
	stop chan *sync.WaitGroup

	currentlyProcessingMU sync.Mutex
	currentlyProcessing   map[int64]bool
	stopBGWorker          chan *sync.WaitGroup
}

func newScheduledEventsPlugin() *ScheduledEvents {
	return &ScheduledEvents{
		stop:                make(chan *sync.WaitGroup),
		currentlyProcessing: make(map[int64]bool),
		stopBGWorker:        make(chan *sync.WaitGroup),
	}
}

func (p *ScheduledEvents) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Scheduled Events",
		SysName:  "scheduled_events",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {
	common.InitSchemas("scheduledevents2", DBSchemas...)

	common.RegisterPlugin(newScheduledEventsPlugin())
}

type HandlerFunc func(evt *models.ScheduledEvent, data interface{}) (retry bool, err error)

type RegisteredHandler struct {
	EvtName    string
	DataFormat interface{}
	Handler    HandlerFunc
}

var (
	registeredHandlers = make(map[string]*RegisteredHandler)
	running            bool
	logger             = common.GetPluginLogger(&ScheduledEvents{})
)

// RegisterHandler registers a handler for the scpecified event name
// dataFormat is optional and should not be a pointer, it should match the type you're passing into ScheduleEvent
func RegisterHandler(eventName string, dataFormat interface{}, handler HandlerFunc) {
	if running {
		panic("tried adding handler when scheduledevents2 is running")
	}

	registeredHandlers[eventName] = &RegisteredHandler{
		EvtName:    eventName,
		DataFormat: dataFormat,
		Handler:    handler,
	}

	logger.Debug("Registered handler for ", eventName)
}

func ScheduleEvent(evtName string, guildID int64, runAt time.Time, data interface{}) error {
	m := &models.ScheduledEvent{
		TriggersAt: runAt,
		EventName:  evtName,
		GuildID:    guildID,
	}

	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return errors.WithMessage(err, "marshal")
		}

		m.Data = b
	} else {
		m.Data = []byte("{}")
	}

	err := m.InsertG(context.Background(), boil.Infer())
	if err != nil {
		return errors.WithStackIf(err)
	}

	if time.Now().Add(time.Hour).After(runAt) {
		err = flushEventToRedis(common.RedisPool, m)
	}

	return errors.WithMessage(err, "insert")
}

var _ bot.LateBotInitHandler = (*ScheduledEvents)(nil)
var _ bot.BotStopperHandler = (*ScheduledEvents)(nil)

func (se *ScheduledEvents) LateBotInit() {
	registerBuiltinEvents()
	running = true
	go se.runCheckLoop()
	go se.MigrateLegacyEvents()
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

var metricsScheduledEventsProcessed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_scheduledevents_processed_total",
	Help: "Total scheduled events processed",
})

var metricsScheduledEventsSkipped = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_scheduledevents_skipped_total",
	Help: "Total scheduled events skipped",
})

func (se *ScheduledEvents) check() {
	se.currentlyProcessingMU.Lock()
	defer se.currentlyProcessingMU.Unlock()

	var pairs []string
	err := common.RedisPool.Do(radix.FlatCmd(&pairs, "ZRANGEBYSCORE", "scheduled_events_soon", 0, time.Now().UTC().Unix()))
	if err != nil {
		logger.WithError(err).Error("failed checking for scheduled events to process")
		return
	}

	numSkipped := 0
	numHandling := 0
	for _, pair := range pairs {
		id, guildID, err := parseIDGuildIDPair(pair)
		if err != nil {
			logger.WithError(err).Error("failed parsing id guildId pair")
			continue
		}

		skip, remove := se.checkShouldSkipRemove(id, guildID)
		if skip && !remove {
			numSkipped++
			continue
		}

		if remove {
			logger.WithField("id", id).WithField("guild", guildID).Info("removing event entirely since it's not on this bot anymore")
			go se.markDoneID(id, guildID, bot.ErrGuildNotOnProcess)
			numSkipped++
			continue
		}

		numHandling++
		se.currentlyProcessing[id] = true
		go se.processItem(id, guildID)
	}

	metricsScheduledEventsProcessed.Add(float64(numHandling))
	metricsScheduledEventsSkipped.Add(float64(numSkipped))

	if numHandling > 0 {
		logger.Info("triggered ", numHandling, " scheduled events (skipped ", numSkipped, ")")
	}
}

func (se *ScheduledEvents) checkShouldSkipRemove(id int64, guildID int64) (skip bool, remove bool) {
	if !bot.ReadyTracker.IsGuildShardReady(guildID) {
		return true, false
	}

	// make sure the guild is available
	gs := bot.State.Guild(true, guildID)
	if gs == nil {
		onGuild, err := common.BotIsOnGuild(guildID)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("failed checking if bot is on guild")
			return true, false
		} else if !onGuild {
			return true, true
		}

		return true, false
	}

	gs.RLock()
	unavailable := gs.Guild.Unavailable
	gs.RUnlock()

	if unavailable {
		// wait until the guild is available before handling this event
		return true, false
	}

	if v := se.currentlyProcessing[id]; v {
		return true, false
	}

	return false, false
}

var ErrBadPairLength = errors.NewPlain("ID - GuildID pair corrupted")

func parseIDGuildIDPair(pair string) (id int64, guildID int64, err error) {
	split := strings.Split(pair, ":")
	if len(split) < 2 {
		err = ErrBadPairLength
		return
	}

	id, err = strconv.ParseInt(split[0], 10, 64)
	if err != nil {
		return
	}

	guildID, err = strconv.ParseInt(split[1], 10, 64)
	return
}

func (se *ScheduledEvents) processItem(id int64, guildID int64) {
	l := logger.WithField("id", id).WithField("guild", guildID)

	defer func() {
		se.currentlyProcessingMU.Lock()
		defer se.currentlyProcessingMU.Unlock()

		delete(se.currentlyProcessing, id)
	}()

	item, err := models.FindScheduledEventG(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			se.markDoneID(id, guildID, nil)
		} else {
			l.WithError(err).Error("failed finding scheduled event")
		}
		return
	}

	if item.Processed {
		se.markDoneID(id, guildID, nil)
		return
	}

	// check if this event was changed after it was flushed
	delta := item.TriggersAt.Sub(time.Now())
	if delta > 5 {
		// it was changed, re-flush it, or remove it
		err = UpdateFlushedEvent(time.Now(), common.RedisPool, item)
		if err != nil {
			logger.WithError(err).Error("failed re-flushing event")
			return
		}
	}

	handler, ok := registeredHandlers[item.EventName]
	if !ok {
		l.Error("unknown event: ", item.EventName)
		se.markDone(item, errors.NewPlain("No registered handler"))
		return
	}

	var decodedData interface{}
	if handler.DataFormat != nil {
		typ := reflect.TypeOf(handler.DataFormat)

		// Decode the form into the destination struct
		decodedData = reflect.New(typ).Interface()
		err := json.Unmarshal(item.Data, decodedData)
		if err != nil {
			l.WithError(err).Error("failed decoding event data")
			se.markDone(item, errors.WithMessage(err, "json"))
			return
		}
	}

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			l.Errorf("recovered from panic in scheduled event handler \n%v\n%v", r, stack)
		}
	}()

	retryDelay := time.Second
	for nRetry := 0; nRetry < 10; nRetry++ {
		var retry bool
		retry, err = handler.Handler(item, decodedData)
		if err != nil {
			l.WithError(err).Error("handler returned an error")
		}

		if retry {
			l.WithError(err).Warn("retrying handling event")
			time.Sleep(retryDelay)
			retryDelay *= 2
			if retryDelay > time.Second*10 {
				retryDelay = time.Second * 10
			}
			continue
		}

		break
	}

	se.markDone(item, err)
}

func (se *ScheduledEvents) markDone(item *models.ScheduledEvent, runErr error) {
	defer func() {
		se.currentlyProcessingMU.Lock()
		delete(se.currentlyProcessing, item.ID)
		se.currentlyProcessingMU.Unlock()
	}()

	item.Processed = true
	if runErr != nil {
		item.Error = null.StringFrom(runErr.Error())
	}
	_, err := item.UpdateG(context.Background(), boil.Whitelist("processed"))

	if err != nil {
		logger.WithError(err).Error("failed marking item as processed")
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZREM", "scheduled_events_soon", fmt.Sprintf("%d:%d", item.ID, item.GuildID)))
	if err != nil {
		logger.WithError(err).Error("failed marking item as done in redis")
	}
}

func (se *ScheduledEvents) markDoneID(id int64, guildID int64, runErr error) {
	// item.Processed = true
	// if runErr != nil {
	// 	item.Error = null.StringFrom(runErr.Error())
	// }
	// _, err := item.UpdateG(context.Background(), boil.Whitelist("processed"))

	var updateErr null.String
	if runErr != nil {
		updateErr = null.StringFrom(runErr.Error())
	}

	defer func() {
		se.currentlyProcessingMU.Lock()
		delete(se.currentlyProcessing, id)
		se.currentlyProcessingMU.Unlock()
	}()

	const q = "UPDATE scheduled_events SET processed=true, error=$2 WHERE id=$1"
	_, err := common.PQ.Exec(q, id, updateErr)
	if err != nil {
		logger.WithError(err).Error("failed marking item as done")
		return
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZREM", "scheduled_events_soon", fmt.Sprintf("%d:%d", id, guildID)))
	if err != nil {
		logger.WithError(err).Error("failed marking item as done")
		return
	}

}

func CheckDiscordErrRetry(err error) bool {
	if err == nil {
		return false
	}

	err = errors.Cause(err)

	if cast, ok := err.(*discordgo.RESTError); ok {
		if cast.Message != nil && cast.Message.Code != 0 {
			// proper discord response, don't retry
			return false
		}
	}

	if err == bot.ErrGuildNotFound {
		return false
	}

	// an unknown error unrelated to the discord api occured (503's for example) attempt a retry
	return true
}
