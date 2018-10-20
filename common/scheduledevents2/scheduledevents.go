package scheduledevents2

//go:generate sqlboiler psql

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/sirupsen/logrus"
)

type ScheduledEvents struct{}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Fatal("scheduledevents failed initializing db schema")
	}

	common.RegisterPlugin(&ScheduledEvents{})
}

func (se *ScheduledEvents) Name() string {
	return "scheduled_events"
}

type HandlerFunc func(evt *models.ScheduledEvent, data interface{}) (err error, retry bool)

type RegisteredHandler struct {
	EvtName    string
	DataFormat interface{}
	Handler    HandlerFunc
}

var (
	registeredHandlers = make(map[string]*RegisteredHandler)
)

func RegisterHandler(eventName string, dataDest interface{}, handler HandlerFunc) {
	registeredHandlers[eventName] = &RegisterHandler{
		EvtName:    eventName,
		DataFormat: dataDest,
		Handler:    handler,
	}

	logrus.Debug("[ScheduledEvents2] Registered handler for ", eventName)
}

var _ BotStartedHandler = (*ScheduledEvents)(nil)

func (se *ScheduledEvents) BotStarted() {
	go se.runCheckLoop()
}

func (se *ScheduledEvents) runCheckLoop() {

}
