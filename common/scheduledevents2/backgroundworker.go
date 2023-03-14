package scheduledevents2

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

const flushTresholdMinutes = 5

var _ backgroundworkers.BackgroundWorkerPlugin = (*ScheduledEvents)(nil)

func (p *ScheduledEvents) RunBackgroundWorker() {

	go p.SecondaryCleaner()

	checkNewEvents := time.NewTicker(time.Minute)

	for {
		select {
		case wg := <-p.stopBGWorker:
			wg.Done()
			return
		case <-checkNewEvents.C:
			logger.Info("Flushing new events...")
			err := runFlushNewEvents()
			if err != nil {
				logger.WithError(err).Error("failed moving scheduled events into redis")
			}
			logger.Info("DONE flushing new events...")
		}
	}
}

func (p *ScheduledEvents) SecondaryCleaner() {
	// dont want our cleanup jobs interfering with our other jobs
	cleanupTicker := time.NewTicker(time.Hour)
	cleanupRecentTicker := time.NewTicker(time.Minute)
	for {
		select {
		case wg := <-p.stopBGWorker:
			wg.Done()
			return
		case <-cleanupTicker.C:
			logger.Info("running generic cleanup...")
			runCleanup()
			logger.Info("DONE running generic cleanup...")
		case <-cleanupRecentTicker.C:
			logger.Info("cleaning up recent events...")
			err := cleanupRecent()
			if err != nil {
				logger.WithError(err).Error("failed cleaning up recent scheduled events")
			}
			logger.Info("DONE cleaning up recent events...")
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
			isDone := false
			c.Do(radix.FlatCmd(&isDone, "SISMEMBER", "recently_done_scheduled_events", v.ID))
			if isDone {
				continue
			}

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
	err := c.Do(radix.Cmd(nil, "ZADD", "scheduled_events_soon", strconv.FormatInt(evt.TriggersAt.UTC().UnixMicro(), 10), v))
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

func cleanupRecent() error {
	var recent []int64
	err := common.RedisPool.Do(radix.Cmd(&recent, "SMEMBERS", "recently_done_scheduled_events"))
	if err != nil {
		return err
	}

	if len(recent) < 1 {
		return nil
	}

	logger.Infof("got %d recent events to clean up...", len(recent))

	if len(recent) < 100 {
		return cleanupRecentBatch(recent)
	}

	i := 0
	for {
		var batch []int64
		if i+100 >= len(recent) {
			batch = recent[i:]
		} else {
			batch = recent[i : i+100]
		}

		err := cleanupRecentBatch(batch)
		if err != nil {
			return err
		}
		i += 100
		if i >= len(recent) {
			break
		}
	}

	return nil
}

func cleanupRecentBatch(ids []int64) error {
	sqlArgs := make([]interface{}, len(ids))
	for i, v := range ids {
		sqlArgs[i] = v
	}

	result, err := models.ScheduledEvents(qm.WhereIn("id in ?", sqlArgs...)).DeleteAll(context.Background(), common.PQ)
	if err != nil {
		return err
	}

	logger.Infof("Deleted %d recently done events", result)

	args := make([]string, len(ids)+1)
	for i, v := range ids {
		args[i+1] = strconv.FormatInt(v, 10)
	}

	args[0] = "recently_done_scheduled_events"

	return common.RedisPool.Do(radix.Cmd(nil, "SREM", args...))
}
