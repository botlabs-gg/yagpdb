package scheduledevents2

import (
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
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
	numSuccess := 0
	numError := 0

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
			numError++
			continue
		}

		err = handler(t, dataPart)
		if err != nil {
			logrus.WithError(err).Error("[scheduledevents2] failed migrating scheduled event: ", fullEvent)
			skipScore++
			numError++
			continue
		}

		// remove it
		common.RedisPool.Do(radix.Cmd(nil, "ZREM", "scheduled_events", fullEvent))
		logrus.Info("[scheduledevents2] successfully migrated ", fullEvent)
		numSuccess++
	}

	if numSuccess > 0 || numError > 0 {
		logrus.Infof("[scheduledevents2] Suscessfully migrated %d scheduled events, failed %d", numSuccess, numError)
	}
}
