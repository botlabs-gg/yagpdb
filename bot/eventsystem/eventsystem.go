package eventsystem

//go:generate go run gen/events_gen.go -o events.go

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/sirupsen/logrus"
)

var DiscordState *dstate.State

func init() {
	for i, _ := range handlers {
		handlers[i] = make([][]*Handler, 3)
	}
}

type HandlerFunc func(evtData *EventData) (retry bool, err error)
type HandlerFuncLegacy func(evtData *EventData)

type Handler struct {
	Plugin  common.Plugin
	F       HandlerFunc
	FLegacy HandlerFuncLegacy
}

type EventData struct {
	EvtInterface      interface{}
	Type              Event
	ctx               context.Context
	Session           *discordgo.Session
	GuildFeatureFlags []string

	GS *dstate.GuildState // Guaranteed to be available for guild events, except creates and deletes
	cs *dstate.ChannelState

	cancelled *int32

	l sync.Mutex
}

func NewEventData(session *discordgo.Session, t Event, evtInterface interface{}) *EventData {
	return &EventData{
		EvtInterface: evtInterface,
		Type:         t,
		Session:      session,
		cancelled:    new(int32),
	}
}
func (e *EventData) Cancel() {
	atomic.StoreInt32(e.cancelled, 1)
}

func (e *EventData) Context() context.Context {
	if e.ctx == nil {
		return context.Background()
	}

	return e.ctx
}

func (e *EventData) WithContext(ctx context.Context) *EventData {
	cop := new(EventData)
	*cop = *e
	cop.ctx = ctx
	return cop
}

// HasFeatureFlag returns true if the guild the event came from has the provided feature flag
func (e *EventData) HasFeatureFlag(flag string) bool {
	return common.ContainsStringSlice(e.GuildFeatureFlags, flag)
}

// EmitEvent emits an event
func EmitEvent(data *EventData, evt Event) {
	h := handlers[evt]

	runEvents(h[0], data)
	runEvents(h[1], data)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				logrus.WithField(logrus.ErrorKey, err).WithField("evt", data.Type.String()).Error("Recovered from panic in event handler\n" + stack)
			}
		}()

		runEvents(h[2], data)
	}()
}

func runEvents(h []*Handler, data *EventData) {

	retryCount := 0
	for _, v := range h {
		retry := true
		sleepTime := 500 * time.Millisecond
		first := true
		for retry && retryCount < 5 {
			if atomic.LoadInt32(data.cancelled) != 0 {
				return
			}

			// Sleep a bit between retries
			// will retry up to 5 times (rc = 4)
			// total time would be 8 seconds
			if retry && !first {
				retryCount++
				time.Sleep(sleepTime)
				sleepTime *= 2
			}

			first = false

			if v.F != nil {
				var err error
				retry, err = v.F(data)

				guildID := int64(0)
				if guildIDProvider, ok := data.EvtInterface.(discordgo.GuildEvent); ok {
					guildID = guildIDProvider.GetGuildID()
				}
				if err != nil {
					logrus.WithField("guild", guildID).WithField("evt", data.Type.String()).Errorf("%s: An error occured in a discord event handler: %+v", v.Plugin.PluginInfo().SysName, err)
				}

				if retry {
					logrus.WithField("guild", guildID).WithField("evt", data.Type.String()).Errorf("%s: Retrying event handler... %dc", v.Plugin.PluginInfo().SysName, retryCount)
				}

			} else {
				retry = false
				v.FLegacy(data)
			}

		}
	}
}

type Order int

const (
	// Ran first, syncrounously, before changes from the event is applied to state
	OrderSyncPreState Order = 0
	// Ran second, syncrounsly, after state changes have been applied
	OrderSyncPostState Order = 1
	// Ran last, asyncrounously, most handlers should use this unless you need something else in special circumstances
	OrderAsyncPostState Order = 2
)

// AddHandler adds a event handler
func AddHandlerLegacy(p common.Plugin, handler HandlerFuncLegacy, order Order, evts ...Event) {
	h := &Handler{
		FLegacy: handler,
		Plugin:  p,
	}

	// check if one of them is EventAll
	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				handlers[e][int(order)] = append(handlers[e][int(order)], h)
			}

			// If one of the events is all, then there's not point in passing more
			return
		}
	}

	for _, evt := range evts {
		handlers[evt][int(order)] = append(handlers[evt][int(order)], h)
	}
}

// AddHandlerFirst adds handlers using the OrderSyncPreState order
func AddHandlerFirstLegacy(p common.Plugin, handler HandlerFuncLegacy, evts ...Event) {
	AddHandlerLegacy(p, handler, OrderSyncPreState, evts...)
}

// AddHandlerSecond adds handlers using the OrderSyncPostState order
func AddHandlerSecondLegacy(p common.Plugin, handler HandlerFuncLegacy, evts ...Event) {
	AddHandlerLegacy(p, handler, OrderSyncPostState, evts...)
}

// AddHandlerAsyncLast adds handlers using the OrderAsyncPostState order
func AddHandlerAsyncLastLegacy(p common.Plugin, handler HandlerFuncLegacy, evts ...Event) {
	AddHandlerLegacy(p, handler, OrderAsyncPostState, evts...)
}

// AddHandler adds a event handler
func AddHandler(p common.Plugin, handler HandlerFunc, order Order, evts ...Event) {
	h := &Handler{
		F:      handler,
		Plugin: p,
	}

	// check if one of them is EventAll
	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				handlers[e][int(order)] = append(handlers[e][int(order)], h)
			}

			// If one of the events is all, then there's not point in passing more
			return
		}
	}

	for _, evt := range evts {
		handlers[evt][int(order)] = append(handlers[evt][int(order)], h)
	}
}

// AddHandlerFirst adds handlers using the OrderSyncPreState order
func AddHandlerFirst(p common.Plugin, handler HandlerFunc, evts ...Event) {
	AddHandler(p, handler, OrderSyncPreState, evts...)
}

// AddHandlerSecond adds handlers using the OrderSyncPostState order
func AddHandlerSecond(p common.Plugin, handler HandlerFunc, evts ...Event) {
	AddHandler(p, handler, OrderSyncPostState, evts...)
}

// AddHandlerAsyncLast adds handlers using the OrderAsyncPostState order
func AddHandlerAsyncLast(p common.Plugin, handler HandlerFunc, evts ...Event) {
	AddHandler(p, handler, OrderAsyncPostState, evts...)
}

func HandleEvent(s *discordgo.Session, evt interface{}) {
	var evtData = &EventData{
		Session:      s,
		EvtInterface: evt,
		cancelled:    new(int32),
	}

	ctx := context.WithValue(context.Background(), common.ContextKeyDiscordSession, s)
	evtData.ctx = ctx

	fillEvent(evtData)

	if s == nil {
		handleEvent(evtData)
		return
	}

	if s.ShardID >= len(workers) || workers[s.ShardID] == nil {
		logrus.Errorf("bad shard event: sid: %d, len: %d", s.ShardID, len(workers))
		return
	}

	select {
	case workers[s.ShardID] <- evtData:
		return
	default:
		// go common.SendOwnerAlert("Max events in queue!")
		logrus.Errorf("Max events in queue: %d, %d", len(workers[s.ShardID]), s.ShardID)
		workers[s.ShardID] <- evtData // attempt to send it anyways for now
	}
}

func QueueEventNonDiscord(evtData *EventData) {
	if evtData.Session != nil {
		ctx := context.WithValue(evtData.Context(), common.ContextKeyDiscordSession, evtData.Session)
		evtData.ctx = ctx
	} else {
		handleEvent(evtData)
		return
	}

	s := evtData.Session
	if s.ShardID >= len(workers) || workers[s.ShardID] == nil {
		logrus.Errorf("bad shard event: sid: %d, len: %d", s.ShardID, len(workers))
		return
	}

	select {
	case workers[s.ShardID] <- evtData:
		return
	default:
		// go common.SendOwnerAlert("Max events in queue!")
		logrus.Errorf("Max events in queue: %d, %d", len(workers[s.ShardID]), s.ShardID)
		workers[s.ShardID] <- evtData // attempt to send it anyways for now
	}
}

// CS will attempt to fetch the channel state from either cached, or state, or return nil if nonexistent (e.g a channel create before the state has been populated by it)
func (d *EventData) CS() *dstate.ChannelState {
	d.l.Lock()
	defer d.l.Unlock()

	if d.cs != nil {
		return d.cs
	}

	if channelEvt, ok := d.EvtInterface.(discordgo.ChannelEvent); ok {
		d.cs = DiscordState.Channel(true, channelEvt.GetChannelID())
	}

	return d.cs
}

// RequireCSMW will only call the inner handler if a channel state is available
func RequireCSMW(inner HandlerFunc) HandlerFunc {
	return func(evt *EventData) (retry bool, err error) {
		if evt.CS() == nil {
			return false, nil
		}

		return inner(evt)
	}
}

var workers []chan *EventData

func InitWorkers(totalShards int) {

	workers = make([]chan *EventData, totalShards)
	for i, _ := range workers {
		workers[i] = make(chan *EventData, 5000)
		go eventWorker(workers[i])
	}
}

func eventWorker(ch chan *EventData) {
	for evt := range ch {
		handleEvent(evt)
	}
}

func handleEvent(evtData *EventData) {
	// fill in guild state if applicable
	if guildEvt, ok := evtData.EvtInterface.(discordgo.GuildEvent); ok {
		id := guildEvt.GetGuildID()
		if id != 0 {
			evtData.GS = DiscordState.Guild(true, id)

			// If guild state is not available for any guild related events, except creates and deletes, do not run the handlers
			if evtData.GS == nil && evtData.Type != EventGuildCreate && evtData.Type != EventGuildDelete {
				logrus.Debugf("Skipped event as guild state info is not available: %v, %d", evtData.Type, guildEvt.GetGuildID())
				return
			}

			flags, err := featureflags.RetryGetGuildFlags(id)
			if err == nil {
				evtData.GuildFeatureFlags = flags
			}
		}
	}

	// attempt to fill in channel state if applicable
	if channelEvt, ok := evtData.EvtInterface.(discordgo.ChannelEvent); ok {
		evtData.cs = DiscordState.Channel(true, channelEvt.GetChannelID())
	}

	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			logrus.WithField(logrus.ErrorKey, err).WithField("evt", evtData.Type.String()).Error("Recovered from panic in event handler\n" + stack)
		}
	}()

	EmitEvent(evtData, EventAllPre)
	EmitEvent(evtData, evtData.Type)
	EmitEvent(evtData, EventAllPost)

}
