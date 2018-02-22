package eventsystem

//go:generate go run gen/events_gen.go -o events.go

import (
	"context"
	"github.com/Sirupsen/logrus"
	"runtime/debug"
)

type Handler func(evtData *EventData)

type EventData struct {
	*EventDataContainer
	EvtInterface interface{}
	Type         Event
	ctx          context.Context
}

var (
	ConcurrentAfter *Handler
)

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
	for i, v := range handlers[evt] {
		(*v)(data)

		// Check if we should start firing the rest in a different goroutine
		if ConcurrentAfter != nil && ConcurrentAfter == v {
			go func(startFrom int) {
				defer func() {
					if err := recover(); err != nil {
						stack := string(debug.Stack())
						logrus.WithField(logrus.ErrorKey, err).WithField("evt", data.Type.String()).Error("Recovered from panic in event handler\n" + stack)
					}
				}()

				for j := startFrom; j < len(handlers[evt]); j++ {
					(*handlers[evt][j])(data)
				}
			}(i + 1)

			break
		}
	}
}

// AddHandler adds a event handler
func AddHandler(handler Handler, evts ...Event) *Handler {
	hPtr := &handler

	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				handlers[e] = append(handlers[e], hPtr)
			}

			// If one of the events is all, then there's not point in passing more
			return hPtr
		}
	}

	for _, evt := range evts {
		handlers[evt] = append(handlers[evt], hPtr)
	}

	return hPtr
}

// AddHandlerFirst adds a handler first in the queue
func AddHandlerFirst(handler Handler, evt Event) *Handler {
	hPtr := &handler
	if evt == EventAll {
		for _, e := range AllDiscordEvents {
			handlers[e] = append([]*Handler{hPtr}, handlers[e]...)
		}
		return hPtr
	}

	handlers[evt] = append([]*Handler{hPtr}, handlers[evt]...)
	return hPtr
}

// AddHandlerBefore adds a handler to be called before another handler
func AddHandlerBefore(handler Handler, evt Event, before *Handler) *Handler {
	hPtr := &handler

	addHandlerBefore(hPtr, evt, before)

	return hPtr
}

func addHandlerBefore(handler *Handler, evt Event, before *Handler) {
	if evt == EventAll {
		for _, e := range AllDiscordEvents {
			addHandlerBefore(handler, e, before)
		}
		return
	}

	for k, v := range handlers[evt] {
		if v == before {
			// Make a copy with the first half in
			handlersCop := make([]*Handler, len(handlers[evt])+1)
			copy(handlersCop, handlers[evt][:k])

			// insert the handler
			handlersCop[k] = handler

			// add the other half
			for i := k; i < len(handlers[evt]); i++ {
				handlersCop[i+1] = handlers[evt][i]
			}

			handlers[evt] = handlersCop

			return
		}
	}

	logrus.Error("Unable to add handler before other handler", handler, before)

	// Not found, just add to end
	handlers[evt] = append(handlers[evt], handler)
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
