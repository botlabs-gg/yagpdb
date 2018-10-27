package scheduledevents2

import (
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

var (
	registeredMigraters = make(map[string]func(t time.Time, data string) error)
)

// RegisterHandler registers a handler for the scpecified event name
// dataFormat is optional and should not be a pointer, it should match the type you're passing into ScheduleEvent
func RegisterLegacyMigrater(eventName string, migrationHandler func(t time.Time, data string) error) {
	registeredMigraters[eventName] = migrationHandler

	logrus.Debug("[scheduledEvents2] Registered migration handler for ", eventName)
}

func (se *ScheduledEvents) MigrateLegacyEvents() {
	if bot.TotalShardCount != bot.ProcessShardCount {
		// to migrate events from the legacy system, we need to have all the channels in the state to be able to determine what guild the events are for
		logrus.Warn("[scheduledevents2] not running all shards in this process, can't migrate scheduled events from the legacy system (ignore if there are none to actually migrate)")
		return
	}

	skipScore := 0
	for {
		var result []string
		err := common.RedisPool.Do(radix.FlatCmd(&result, "ZREVRANGE", "scheduled_events", skipScore, skipScore, "WITHSCORES"))
		if err != nil {
			logrus.WithError(err).Error("[scheduledevents2] failed migrating scheduledevents")
			break
		}

		if len(result) < 2 {
			logrus.Info("[scheduledevents2] done migrating legacy events to new format")
			break
		}

		fullEvent := result[0]
		score, _ := strconv.ParseInt(result[1], 10, 64)
		t := time.Unix(score, 0)

		split := strings.SplitN(fullEvent, ":", 2)
		dataPart := ""
		if len(split) > 1 {
			dataPart = split[1]
		}

		handler, ok := registeredMigraters[split[0]]
		if !ok {
			logrus.Error("[scheduledevents2] no migrater found for event: ", split[0])
			skipScore++
			continue
		}

		err = handler(t, dataPart)
		if err != nil {
			logrus.WithError(err).Error("[scheduledevents2] failed migrating scheduled event: ", fullEvent)
			skipScore++
			continue
		}

		// remove it
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "scheduled_events", fullEvent))
		logrus.Info("[scheduledevents2] successfully migrated ", fullEvent)
	}
}
