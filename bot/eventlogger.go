package bot

import (
	"context"
	"github.com/Sirupsen/logrus"
	"reflect"
	"sync"
)

var (
	EventLogger = &eventLogger{Events: make(map[string]int)}
)

type eventLogger struct {
	sync.Mutex
	Events map[string]int
}

func (e *eventLogger) handleEvent(ctx context.Context, evt interface{}) {
	evtName := reflect.Indirect(reflect.ValueOf(evt)).Type().Name()

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
