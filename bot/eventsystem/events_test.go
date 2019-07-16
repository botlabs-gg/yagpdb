package eventsystem

import (
	"testing"

	"github.com/jonas747/discordgo"
)

func TestAddHandlerOrder(t *testing.T) {
	firstTriggered := false
	h1 := func(evt *EventData) {
		firstTriggered = true
	}
	h2 := func(evt *EventData) {
		if !firstTriggered {
			t.Error("Unordered!")
		}
	}

	AddHandlerSecond(h2, EventReady)
	AddHandlerFirst(h1, EventReady)
	HandleEvent(nil, &discordgo.Ready{})
}
