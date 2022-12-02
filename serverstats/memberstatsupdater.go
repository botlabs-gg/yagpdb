package serverstats

import (
	"database/sql"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/mediocregopher/radix/v3"
)

type serverMemberStatsUpdater struct {
	flushInterval time.Duration
	incoming      chan *eventsystem.EventData

	waiting    []*QueuedAction
	processing []*QueuedAction

	flushInProgress *int32
}

func newServerMemberStatsUpdater() *serverMemberStatsUpdater {
	return &serverMemberStatsUpdater{
		flushInterval:   time.Minute * 10,
		incoming:        make(chan *eventsystem.EventData),
		flushInProgress: new(int32),
	}
}

type QueuedAction struct {
	GuildID    int64
	TotalCount int
	Joins      int
	Leaves     int
}

func (mu *serverMemberStatsUpdater) run() {

	t := time.NewTicker(mu.flushInterval)
	for {
		select {
		case <-t.C:
			if atomic.LoadInt32(mu.flushInProgress) != 0 {
				logger.Error("last flush took too long, waiting...")
				continue
			}

			if len(mu.waiting) == 0 && len(mu.processing) == 0 {
				continue
			}

			logger.Infof("Flushing member stats, len:%d, leftovers:%d", len(mu.waiting), len(mu.processing))

			// merge the leftovers form last run into the main queue
		OUTER:
			for _, pv := range mu.processing {
				for _, wv := range mu.waiting {
					if wv.GuildID == pv.GuildID {
						pv.Joins += wv.Joins
						pv.Leaves += wv.Leaves
						continue OUTER
					}
				}

				mu.waiting = append(mu.waiting, pv)
			}

			mu.processing = mu.waiting
			mu.waiting = nil

			atomic.StoreInt32(mu.flushInProgress, 1)
			go mu.flush()
		case evt := <-mu.incoming:
			mu.handleIncEvent(evt)
		}
	}
}

func (mu *serverMemberStatsUpdater) handleIncEvent(evt *eventsystem.EventData) {
	q := QueuedAction{}

	switch evt.Type {
	case eventsystem.EventGuildCreate:
		e := evt.GuildCreate()
		q.GuildID = e.ID
		q.TotalCount = e.MemberCount
	case eventsystem.EventGuildMemberAdd:
		q.GuildID = evt.GS.ID
		q.Joins = 1
	case eventsystem.EventGuildMemberRemove:
		q.GuildID = evt.GS.ID
		q.Leaves = 1
	}

	for _, v := range mu.waiting {
		if v.GuildID == q.GuildID {
			v.Joins += q.Joins
			v.Leaves += q.Leaves
			if q.TotalCount != 0 {
				v.TotalCount = q.TotalCount
			} else if v.TotalCount != 0 {
				v.TotalCount += q.Joins
				v.TotalCount -= q.Leaves
			}

			return
		}
	}

	mu.waiting = append(mu.waiting, &q)
}

func keyTotalMembers(year, day int) string {
	return "serverstats_total_members:" + strconv.Itoa(year) + ":" + strconv.Itoa(day)
}
func keyJoinedMembers(year, day int) string {
	return "serverstats_joined_members:" + strconv.Itoa(year) + ":" + strconv.Itoa(day)
}
func keyLeftMembers(year, day int) string {
	return "serverstats_left_members:" + strconv.Itoa(year) + ":" + strconv.Itoa(day)
}

func (mu *serverMemberStatsUpdater) flush() {
	leftOver := make([]*QueuedAction, 0)
	defer func() {
		mu.processing = leftOver
		atomic.StoreInt32(mu.flushInProgress, 0)
	}()

	sleepBetweenCalls := time.Second
	if len(mu.processing) > 0 {
		sleepBetweenCalls = mu.flushInterval / time.Duration(len(mu.processing))
		sleepBetweenCalls /= 2
	}

	ticker := time.NewTicker(sleepBetweenCalls)
	defer ticker.Stop()

	t := time.Now()
	year := t.Year()
	day := t.YearDay()
	for _, v := range mu.processing {
		if v.TotalCount > 0 {
			err := common.RedisPool.Do(radix.FlatCmd(nil, "ZADD", keyTotalMembers(year, day), v.TotalCount, v.GuildID))
			if err != nil {
				leftOver = append(leftOver, v)
				logger.WithError(err).Error("failed flushing serverstats total members")
				return
			}
		} else if v.Joins > 0 || v.Leaves > 0 {
			// apply a total members change if present
			memberMod := v.Joins - v.Leaves
			if memberMod > 0 {
				err := common.RedisPool.Do(radix.FlatCmd(nil, "ZINCRBY", keyTotalMembers(year, day), memberMod, v.GuildID))
				if err != nil {
					leftOver = append(leftOver, v)
					logger.WithError(err).Error("failed flushing serverstats total changemod")
					return
				}
			}
		}

		if v.Joins > 0 {
			err := common.RedisPool.Do(radix.FlatCmd(nil, "ZINCRBY", keyJoinedMembers(year, day), v.Joins, v.GuildID))
			if err != nil {
				leftOver = append(leftOver, v)
				logger.WithError(err).Error("failed flushing serverstats joins")
				return
			}

			v.Joins = 0
		}

		if v.Leaves > 0 {
			err := common.RedisPool.Do(radix.FlatCmd(nil, "ZINCRBY", keyLeftMembers(year, day), v.Leaves, v.GuildID))
			if err != nil {
				leftOver = append(leftOver, v)
				logger.WithError(err).Error("failed flushing serverstats leaves")
				return
			}

			v.Leaves = 0
		}

		<-ticker.C

	}
}

func (mu *serverMemberStatsUpdater) setUpdateMemberStatsPeriod(tx *sql.Tx, guildID int64, memberIncr int, numMembers int) error {
	joins := 0
	leaves := 0
	if memberIncr > 0 {
		joins = memberIncr
	} else if memberIncr < 0 {
		leaves = -memberIncr
	}

	// round to current hour
	t := RoundHour(time.Now())

	_, err := tx.Exec(`INSERT INTO server_stats_hourly_periods_misc  (guild_id, t, num_members, joins, leaves, max_online, max_voice)
VALUES ($1, $2, $3, $4, $5, 0, 0)
ON CONFLICT (guild_id, t)
DO UPDATE SET 
joins = server_stats_hourly_periods_misc.joins + $4, 
leaves = server_stats_hourly_periods_misc.leaves + $5, 
num_members = server_stats_hourly_periods_misc.num_members + $6;`, guildID, t, numMembers, joins, leaves, memberIncr)

	return err
}
