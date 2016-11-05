package common

// Scheduled events are stored in a redis sorted set, with the score being when they should be triggered in unix time
// It is checked every minute, so can be up to a minute off.
// In the key, everythign after : is ignored, use this to store things like, serverid, playerids or other simple
// data (for example for reminders, you would set it to channelid, and userid)

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"strings"
	"sync"
	"time"
)

type ScheduledEvtHandler func(evt string)

var scheduledHandlers map[string]ScheduledEvtHandler

func RegisterScheduledEventHandler(evt string, handler ScheduledEvtHandler) {
	scheduledHandlers[evt] = handler
}

func ScheduleEvent(client *redis.Client, evt, data string, when time.Time) error {
	reply := client.Cmd("ZADD", "scheduled_events", when.Unix(), evt+":"+data)
	return reply.Err
}

// Checks for and handles scheduled events every minute
func RunScheduledEvents(stop chan *sync.WaitGroup) {
	client, err := RedisPool.Get()
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case wg := <-stop:
			wg.Done()
			return
		case <-ticker.C:
			started := time.Now()
			n, err := checkScheduledEvents(client)
			if err != nil {
				logrus.WithError(err).Error("Failed checking scheduled events")
			}
			logrus.Infof("Handled %d scheduled events in %s", n, time.Since(started))
		}
	}
}

func checkScheduledEvents(client *redis.Client) (int, error) {
	now := time.Now()
	reply := client.Cmd("ZRANGEBYSCORE", "scheduled_events", "-inf", now.Unix())
	evts, err := reply.List()
	if err != nil {
		return 0, err
	}

	for _, v := range evts {
		handleScheduledEvent(v, client)
	}

	err = client.Cmd("ZREMRANGEBYSCORE", "scheduled_events", "-inf", now.Unix()).Err
	return len(evts), err
}

func handleScheduledEvent(evt string, client *redis.Client) {
	split := strings.SplitN(evt, ":", 2)
	rest := ""
	if len(split) > 1 {
		rest = split[1]
	}

	handler, found := scheduledHandlers[split[0]]
	if !found {
		logrus.Warnf("No handler found for scheduled event %q", split[0])
		return
	}

	go handler(rest)
}
