package scheduledevents2

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

const flushTresholdMinutes = time.Duration(60)

var _ backgroundworkers.BackgroundWorkerPlugin = (*ScheduledEvents)(nil)

func (p *ScheduledEvents) RunBackgroundWorker() {
	cleanupTicker := time.NewTicker(time.Hour)
	checkNewEvents := time.NewTicker(time.Minute)

	for {
		select {
		case wg := <-p.stopBGWorker:
			wg.Done()
			return
		case <-cleanupTicker.C:
			runCleanup()
		case <-checkNewEvents.C:
			err := runFlushNewEvents()
			if err != nil {
				logger.WithError(err).Error("failed moving scheduled events into redis")
			}
		}
	}
}

func runCleanup() {
	n, err := models.ScheduledEvents(qm.Where("processed=true")).DeleteAll(context.Background(), common.PQ)
	if err != nil {
		logger.WithError(err).Error("error running cleanup")
	} else {
		logger.Println("cleaned up ", n, " entries")
	}
}

func runFlushNewEvents() error {
	where := qm.Where(fmt.Sprintf("triggers_at < now() + INTERVAL '%d minutes' AND processed=false", flushTresholdMinutes))
	eventsTriggeringSoon, err := models.ScheduledEvents(where).AllG(context.Background())
	if err != nil {
		return errors.WithStackIf(err)
	}

	err = common.RedisPool.Do(radix.WithConn("a", func(c radix.Conn) error {
		for _, v := range eventsTriggeringSoon {
			err := flushEventToRedis(c, v)
			if err != nil {
				return err
			}
		}

		return nil
	}))

	return err
}

// flushEventToRedis flushes an event to redis, this is done as a performance improvement as the postgres db is only queried as often
func flushEventToRedis(c radix.Client, evt *models.ScheduledEvent) error {
	v := fmt.Sprintf("%d:%d", evt.ID, evt.GuildID)
	err := c.Do(radix.Cmd(nil, "ZADD", "scheduled_events_soon", strconv.FormatInt(evt.TriggersAt.UTC().Unix(), 10), v))
	if err != nil {
		return err
	}

	return nil
}

// UpdateFlushedEvent updates a already flushed event by either removing it if its above the treshold, or updating the score
func UpdateFlushedEvent(t time.Time, c radix.Client, evt *models.ScheduledEvent) error {
	delta := evt.TriggersAt.Sub(t)
	if delta < flushTresholdMinutes*time.Minute {
		return flushEventToRedis(c, evt)
	}

	// otherwise delete it
	err := c.Do(radix.Cmd(nil, "ZREM", "scheduled_events_soon", fmt.Sprintf("%d:%d", evt.ID, evt.GuildID)))
	return err
}

func (p *ScheduledEvents) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopBGWorker <- wg
}
