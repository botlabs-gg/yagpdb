package bot

import (
	"context"
)

type ContextKey int

const (
	ContextKeySession ContextKey = iota
)

func EmitEvent(ctx context.Context, id Event, evt interface{}) {
	for _, v := range handlers[id] {
		(*v)(ctx, evt)
	}
}

func AddHandler(handler Handler, evts ...Event) {
	for _, evt := range evts {
		if evt == EventAll {
			for _, e := range AllDiscordEvents {
				AddHandler(handler, e)
			}
			break
		}
		handlers[evt] = append(handlers[evt], &handler)
	}
}
func AddHandlerFirst(handler Handler, evt Event) {
	if evt == EventAll {
		for _, e := range AllDiscordEvents {
			AddHandlerFirst(handler, e)
		}
		return
	}

	handlers[evt] = append([]*Handler{&handler}, handlers[evt]...)
}

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
