package eventsystem

import (
	"testing"

	"github.com/jonas747/discordgo"
)

func TestAddHandlerAfter(t *testing.T) {
	firstTriggered := false
	h1 := func(evt *EventData) {
		firstTriggered = true
	}
	h2 := func(evt *EventData) {
		if !firstTriggered {
			t.Error("Unordered!")
		}
	}

	h2Ptr := AddHandler(h2, EventReady)
	AddHandlerBefore(h1, EventReady, h2Ptr)
	HandleEvent(nil, &discordgo.Ready{})
}
