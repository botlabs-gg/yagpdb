package bot

import (
	"context"
	"testing"
)

func TestAddHandlerAfter(t *testing.T) {
	firstTriggered := false
	h1 := func(ctx context.Context, evt interface{}) {
		firstTriggered = true
	}
	h2 := func(ctx context.Context, evt interface{}) {
		if !firstTriggered {
			t.Error("Unordered!")
		}
	}

	AddHandler(h2, EventReady)
	AddHandlerBefore(h1, EventReady, h2)
	triggerHandlers(context.Background(), EventReady, nil)
}
