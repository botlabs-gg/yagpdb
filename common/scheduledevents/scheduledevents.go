package scheduledevents

// Scheduled events are stored in a redis sorted set, with the score being when they should be triggered in unix time
// It is checked every minute, so can be up to a minute off.
// In the key, everythign after : is ignored, use this to store things like, serverid, playerids or other simple
// data (for example for reminders, you would set it to channelid, and userid)

// LIMITATIONS: different events cannot have same key as another event

import (
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/sirupsen/logrus"
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

func ScheduleEvent(client *redis.Client, evt, data string, when time.Time) error {
	reply := client.Cmd("ZADD", "scheduled_events", when.Unix(), evt+":"+data)
	return reply.Err
}

func RemoveEvent(client *redis.Client, evt, data string) error {
	return client.Cmd("ZREM", "scheduled_events", evt+":"+data).Err
}

var stopScheduledEventsChan = make(chan *sync.WaitGroup)

func Stop(wg *sync.WaitGroup) {
	stopScheduledEventsChan <- wg
}

func NumScheduledEvents(client *redis.Client) (int, error) {
	return client.Cmd("ZCARD", "scheduled_events").Int()
}

// Checks for and handles scheduled events every minute
func Run() {
	client, err := common.RedisPool.Get()
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case wg := <-stopScheduledEventsChan:
			waitStop()
			wg.Done()
			return
		case <-ticker.C:
			started := time.Now()
			n, err := checkScheduledEvents(client)
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

func checkScheduledEvents(client *redis.Client) (int, error) {
	now := time.Now().Unix()
	reply := client.Cmd("ZRANGEBYSCORE", "scheduled_events", "-inf", now)
	evts, err := reply.List()
	if err != nil {
		return 0, err
	}

	err = client.Cmd("ZREMRANGEBYSCORE", "scheduled_events", "-inf", now).Err
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

		client, err := common.RedisPool.Get()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving redis connection from pool")
			return
		}
		defer common.RedisPool.Put(client)

		err = ScheduleEvent(client, split[0], rest, time.Now())
		if err != nil {
			logrus.WithError(err).Error("Failed re-scheduling failed event")
		}
	}
}
