package bot

import (
	"context"
)

type ContextKey int

const (
	ContextKeySession ContextKey = iota
)

// EmitEvent emits an event
func EmitEvent(ctx context.Context, id Event, evt interface{}) {
	for _, v := range handlers[id] {
		(*v)(ctx, evt)
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
