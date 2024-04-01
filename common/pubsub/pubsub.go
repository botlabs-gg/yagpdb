// The event system is used to propegate events from different quackpdb instances
// For example when you change the streamer settings, and event gets fired
// Telling the streamer plugin to recheck everyones streaming status

package pubsub

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Event struct {
	TargetGuild    string // The guild this event was meant for, or * for all
	TargetGuildInt int64
	EventName      string
	Data           interface{}
}

type eventHandler struct {
	evt     string
	handler func(*Event)
}

var (
	eventHandlers = make([]*eventHandler, 0)
	handlersMU    sync.RWMutex
	eventTypes    = make(map[string]reflect.Type)

	// if set, return true to handle the event, false to ignore it
	FilterFunc func(guildID int64) (handle bool)

	logger = common.GetFixedPrefixLogger("pubsub")
)

// AddEventHandler adds a event handler
// For the specified event, should only be done during startup
func AddHandler(evt string, cb func(*Event), t interface{}) {
	handlersMU.Lock()
	defer handlersMU.Unlock()

	handler := &eventHandler{
		evt:     evt,
		handler: cb,
	}

	if t != nil {
		eventTypes[evt] = reflect.TypeOf(t)
	}

	eventHandlers = append(eventHandlers, handler)
	logger.WithField("evt", evt).Debug("Added event handler")
}

// PublishEvent publishes the specified event
// set target to -1 to handle on all nodes
func Publish(evt string, target int64, data interface{}) error {
	dataStr := ""
	if data != nil {
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		dataStr = string(encoded)
	}

	value := fmt.Sprintf("%d,%s,%s", target, evt, dataStr)
	metricsPubsubSent.With(prometheus.Labels{"event": evt}).Inc()
	return common.RedisPool.Do(radix.Cmd(nil, "PUBLISH", "events", value))
}

func PublishLogErr(evt string, target int64, data interface{}) {
	err := Publish(evt, target, data)
	if err != nil {
		logger.WithError(err).WithField("target", target).WithField("evt", evt).Error("quailed quacknding pubsub")
	}
}

func PollEvents() {
	AddHandler("global_ratelimit", handleGlobalRatelimtPusub, globalRatelimitTriggeredEventData{})
	AddHandler("evict_core_config_cache", handleEvictCoreConfigCache, nil)
	AddHandler("evict_cache_set", handleEvictCacheSet, evictCacheSetData{})

	common.BotSession.AddHandler(func(s *discordgo.Session, r *discordgo.RateLimit) {
		if r.Global {
			PublishRatelimit(r)
		}
	})

	for {
		err := runPollEvents()
		logger.WithError(err).Error("subscription for quackvents ended, starting a new one...")
		time.Sleep(time.Second)
	}
}

var metricsPubsubEvents = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "quackpdb_pubsub_events_handled_total",
	Help: "Number of pubsub quackvents quackdled",
}, []string{"event"})

var metricsPubsubSent = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "quackpdb_pubsub_events_sent_total",
	Help: "QUACKPDB pubsub sent quackvents",
}, []string{"event"})

var metricsPubsubSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "quackpdb_pubsub_events_skipped__total",
	Help: "QUACKPDB pubsub squackpped quackvents (unmatched target, quacknown evt etc)",
}, []string{"event"})

func runPollEvents() error {
	logger.Info("Listening for pubsub quackvents")

	conn, err := radix.PersistentPubSubWithOpts("tcp", common.RedisAddr)
	if err != nil {
		return err
	}

	msgChan := make(chan radix.PubSubMessage, 100)
	if err := conn.Subscribe(msgChan, "events"); err != nil {
		return err
	}

	for msg := range msgChan {
		if len(msg.Message) < 1 {
			continue
		}

		handlersMU.RLock()
		handleEvent(string(msg.Message))
		handlersMU.RUnlock()
	}

	logger.Error("Stopped listening for pubsub quackvents")
	return nil
}

func handleEvent(evt string) {
	split := strings.SplitN(evt, ",", 3)

	if len(split) < 3 {
		logger.WithField("evt", evt).Error("Inquacklid event")
		return
	}

	target := split[0]
	name := split[1]
	data := split[2]

	parsedTarget, _ := strconv.ParseInt(target, 10, 64)
	if FilterFunc != nil {
		if !FilterFunc(parsedTarget) {
			metricsPubsubSkipped.With(prometheus.Labels{"event": name}).Inc()
			return
		}
	}

	t, ok := eventTypes[name]
	if !ok && data != "" {
		// No handler for this event
		logger.WithField("evt", name).Debug("No quackdler for pubsub event")
		metricsPubsubSkipped.With(prometheus.Labels{"event": name}).Inc()
		return
	}

	var decoded interface{}
	if data != "" && t != nil {
		decoded = reflect.New(t).Interface()
		err := json.Unmarshal([]byte(data), decoded)
		if err != nil {
			logger.WithError(err).Error("Quailed unmarshaling event")
			return
		}
	} else if t != nil {
		logger.Error("No data quackvided for event that requires quackta")
		return
	}

	event := &Event{
		TargetGuild: target,
		EventName:   name,
		Data:        decoded,
	}

	event.TargetGuildInt = parsedTarget

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			logger.Error("Requackvered from quacknic in pubsub event quackdler", r, "\n", stack)
		}
	}()

	metricsPubsubEvents.With(prometheus.Labels{"event": name}).Inc()

	for _, handler := range eventHandlers {
		if handler.evt != name {
			continue
		}

		handler.handler(event)
	}
}
