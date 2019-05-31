package eventsystem

//go:generate go run gen/events_gen.go -o events.go

import (
	"context"
	"runtime/debug"
	"sync/atomic"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
)

func init() {
	for i, _ := range handlers {
		handlers[i] = make([][]Handler, 3)
	}
}

type Handler func(evtData *EventData)

type EventData struct {
	EvtInterface interface{}
	Type         Event
	ctx          context.Context
	Session      *discordgo.Session

	cancelled *int32
}

func NewEventData(session *discordgo.Session, t Event, evtInterface interface{}) *EventData {
	return &EventData{
		EvtInterface: evtInterface,
		Type:         t,
		Session:      session,
		cancelled:    new(int32),
	}
}
func (evt *EventData) Cancel() {
	atomic.StoreInt32(evt.cancelled, 1)
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

func runEvents(h []Handler, data *EventData) {
	for _, v := range h {
		if atomic.LoadInt32(data.cancelled) != 0 {
			return
		}

		v(data)
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
func AddHandler(handler Handler, order Order, evts ...Event) {
	// check if one of them is EventAll
	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				handlers[e][int(order)] = append(handlers[e][int(order)], handler)
			}

			// If one of the events is all, then there's not point in passing more
			return
		}
	}

	for _, evt := range evts {
		handlers[evt][int(order)] = append(handlers[evt][int(order)], handler)
	}
}

// AddHandlerFirst adds handlers using the OrderSyncPreState order
func AddHandlerFirst(handler Handler, evts ...Event) {
	AddHandler(handler, OrderSyncPreState, evts...)
}

// AddHandlerSecond adds handlers using the OrderSyncPostState order
func AddHandlerSecond(handler Handler, evts ...Event) {
	AddHandler(handler, OrderSyncPostState, evts...)
}

// AddHandlerAsyncLast adds handlers using the OrderAsyncPostState order
func AddHandlerAsyncLast(handler Handler, evts ...Event) {
	AddHandler(handler, OrderAsyncPostState, evts...)
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
