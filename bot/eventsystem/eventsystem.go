package eventsystem

//go:generate go run gen/events_gen.go -o events.go

import (
	"context"
)

type Handler func(evtData *EventData)

type EventData struct {
	*EventDataContainer
	EvtInterface interface{}
	Type         Event
	ctx          context.Context
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
	for _, v := range handlers[evt] {
		(*v)(data)
	}
}

// AddHandler adds a event handler
func AddHandler(handler Handler, evts ...Event) {
	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				AddHandler(handler, e)
			}

			// If one of the events is all, then there's not point in passing more
			return
		}
	}

	for _, evt := range evts {
		handlers[evt] = append(handlers[evt], &handler)
	}
}

// AddHandlerFirst adds a handler first in the queue
func AddHandlerFirst(handler Handler, evt Event) {
	if evt == EventAll {
		for _, e := range AllDiscordEvents {
			AddHandlerFirst(handler, e)
		}
		return
	}

	handlers[evt] = append([]*Handler{&handler}, handlers[evt]...)
}

// AddHandlerBefore adds a handler to be called before another handler
func AddHandlerBefore(handler Handler, evt Event, before Handler) {

	if evt == EventAll {
		for _, e := range AllDiscordEvents {
			AddHandlerBefore(handler, e, before)
		}
		return
	}

	hList := handlers[evt]

	for k, v := range hList {
		if v == &before {
			handlers[evt] = append(hList[:k], &handler)
			handlers[evt] = append(handlers[evt], hList[k:]...)
			return
		}
	}

	// Not found, just add to end
	handlers[evt] = append(handlers[evt], &handler)
}

func NumHandlers(evt Event) int {
	if evt != EventAll {
		return len(handlers[evt])
	}

	total := 0
	for _, v := range handlers {
		total += len(v)
	}
	return total
}
