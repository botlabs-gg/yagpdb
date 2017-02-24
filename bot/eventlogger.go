package bot

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"sync"
)

var (
	EventLogger = &eventLogger{Events: make(map[string]int)}
)

type eventLogger struct {
	sync.Mutex
	Events map[string]int
}

func (e *eventLogger) handleEvent(evt *eventsystem.EventData) {
	evtName := evt.Type.String()

	if evtName == "" {
		logrus.Error("Unknown event handled", evt)
		return
	}

	e.Lock()
	defer e.Unlock()

	if _, ok := e.Events[evtName]; !ok {
		e.Events[evtName] = 1
	} else {
		e.Events[evtName]++
	}
}
