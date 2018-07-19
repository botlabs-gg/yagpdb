package scheduledevents

// Scheduled events are stored in a redis sorted set, with the score being when they should be triggered in unix time
// It is checked every minute, so can be up to a minute off.
// In the key, everythign after : is ignored, use this to store things like, serverid, playerids or other simple
// data (for example for reminders, you would set it to channelid, and userid)

// LIMITATIONS: different events cannot have same key as another event

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// If error is not nil, it will be added back
type EvtHandler func(evt string) error

var handlers = make(map[string]EvtHandler)
var (
	currentlyProcessingHandlers = new(int64)
)

func RegisterEventHandler(evt string, handler EvtHandler) {
	handlers[evt] = handler
}

func ScheduleEvent(evt, data string, when time.Time) error {
	err := common.RedisPool.Do(radix.Cmd(nil, "ZADD", "scheduled_events", strconv.FormatInt(when.Unix(), 10), evt+":"+data))
	return err
}

func RemoveEvent(evt, data string) error {
	return common.RedisPool.Do(radix.Cmd(nil, "ZREM", "scheduled_events", evt+":"+data))
}

var stopScheduledEventsChan = make(chan *sync.WaitGroup)

func Stop(wg *sync.WaitGroup) {
	stopScheduledEventsChan <- wg
}

func NumScheduledEvents() (n int, err error) {
	err = common.RedisPool.Do(radix.Cmd(&n, "ZCARD", "scheduled_events"))
	return
}

// Checks for and handles scheduled events every minute
func Run() {
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case wg := <-stopScheduledEventsChan:
			waitStop()
			wg.Done()
			return
		case <-ticker.C:
			started := time.Now()
			n, err := checkScheduledEvents()
			if err != nil {
				logrus.WithError(err).Error("Failed checking scheduled events")
			}
			if n > 0 {
				logrus.Infof("Handled %d scheduled events in %s", n, time.Since(started))
			}
		}
	}
}

func waitStop() {
	for i := 0; i < 30; i++ {
		current := NumCurrentlyProcessing()
		if current < 1 {
			return
		}
		logrus.Warnf("[%d/%d] %d Scheduled event handlers are still processing...", i, 20, current)
		time.Sleep(time.Second)
	}
}

func NumCurrentlyProcessing() int64 {
	return atomic.LoadInt64(currentlyProcessingHandlers)
}

func checkScheduledEvents() (int, error) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	var evts []string
	err := common.RedisPool.Do(radix.Cmd(&evts, "ZRANGEBYSCORE", "scheduled_events", "-inf", now))
	if err != nil {
		return 0, err
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "ZREMRANGEBYSCORE", "scheduled_events", "-inf", now))
	if err != nil {
		return 0, err
	}

	for _, v := range evts {
		go handleScheduledEvent(v)
	}

	return len(evts), nil
}

func handleScheduledEvent(evt string) {
	split := strings.SplitN(evt, ":", 2)
	rest := ""
	if len(split) > 1 {
		rest = split[1]
	}

	handler, found := handlers[split[0]]
	if !found {
		logrus.Warnf("No handler found for scheduled event %q", split[0])
		return
	}

	atomic.AddInt64(currentlyProcessingHandlers, 1)
	defer atomic.AddInt64(currentlyProcessingHandlers, -1)

	handlerErr := handler(rest)
	// Re-schedule the event if an error occured
	if handlerErr != nil {
		logrus.WithError(handlerErr).WithField("sevt", split[0]).Error("Failed handling scheduled event, re-scheduling.")

		err := ScheduleEvent(split[0], rest, time.Now())
		if err != nil {
			logrus.WithError(err).Error("Failed re-scheduling failed event")
		}
	}
}
