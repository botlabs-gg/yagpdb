// The event system is used to propegate events from different yagpdb instances
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

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
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
	return common.RedisPool.Do(radix.Cmd(nil, "PUBLISH", "events", value))
}

func PollEvents() {
	AddHandler("global_ratelimit", handleGlobalRatelimtPusub, globalRatelimitTriggeredEventData{})
	common.BotSession.AddHandler(func(s *discordgo.Session, r *discordgo.RateLimit) {
		if r.Global {
			PublishRatelimit(r)
		}
	})

	for {
		err := runPollEvents()
		logger.WithError(err).Error("subscription for events ended, starting a new one...")
		time.Sleep(time.Second)
	}
}

func runPollEvents() error {
	logger.Info("Listening for pubsub events")

	client, err := radix.Dial("tcp", common.ConfRedis.GetString())
	if err != nil {
		return err
	}

	defer client.Close()

	pubsubClient := radix.PubSub(client)

	msgChan := make(chan radix.PubSubMessage)
	if err := pubsubClient.Subscribe(msgChan, "events"); err != nil {
		return err
	}

	for msg := range msgChan {
		if len(msg.Message) < 1 {
			continue
		}

		handlersMU.RLock()
		logger.WithField("evt", string(msg.Message)).Debug("Handling PubSub event")
		handleEvent(string(msg.Message))
		handlersMU.RUnlock()
	}

	logger.Info("Stopped listening for pubsub events")
	return nil
}

func handleEvent(evt string) {
	split := strings.SplitN(evt, ",", 3)

	if len(split) < 3 {
		logger.WithField("evt", evt).Error("Invalid event")
		return
	}

	target := split[0]
	name := split[1]
	data := split[2]

	parsedTarget, _ := strconv.ParseInt(target, 10, 64)
	if FilterFunc != nil {
		if !FilterFunc(parsedTarget) {
			return
		}
	}

	t, ok := eventTypes[name]
	if !ok && data != "" {
		// No handler for this event
		logger.WithField("evt", name).Debug("No handler for pubsub event")
		return
	}

	var decoded interface{}
	if data != "" && t != nil {
		decoded = reflect.New(t).Interface()
		err := json.Unmarshal([]byte(data), decoded)
		if err != nil {
			logger.WithError(err).Error("Failed unmarshaling event")
			return
		}
	} else if t != nil {
		logger.Error("No data provided for event that requires data")
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
			logger.Error("Recovered from panic in pubsub event handler", r, "\n", stack)
		}
	}()

	for _, handler := range eventHandlers {
		if handler.evt != name {
			continue
		}

		handler.handler(event)
	}
}
