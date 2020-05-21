package serverstats

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
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
		flushInterval:   time.Minute,
		incoming:        make(chan *eventsystem.EventData),
		flushInProgress: new(int32),
	}
}

type QueuedAction struct {
	GuildID        int64
	MemberCountMod int
	TotalCount     int
}

func (mu *serverMemberStatsUpdater) run() {

	t := time.NewTicker(time.Minute * 5)
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
						wv.MemberCountMod += wv.MemberCountMod
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
		q.MemberCountMod = 1
	case eventsystem.EventGuildMemberRemove:
		q.GuildID = evt.GS.ID
		q.MemberCountMod = -1
	}

	for _, v := range mu.waiting {
		if v.GuildID == q.GuildID {
			v.MemberCountMod += q.MemberCountMod
			if q.TotalCount != 0 {
				v.TotalCount = q.TotalCount
			} else if v.TotalCount != 0 {
				v.TotalCount += q.MemberCountMod
			}

			return
		}
	}

	mu.waiting = append(mu.waiting, &q)
}

func (mu *serverMemberStatsUpdater) flush() {
	leftOver := make([]*QueuedAction, 0)
	defer func() {
		mu.processing = leftOver
		atomic.StoreInt32(mu.flushInProgress, 0)
	}()

	// fill in total counts, do this before creating tx to avoid deadlocks
	for _, q := range mu.processing {
		if q.TotalCount == 0 {
			gs := bot.State.Guild(true, q.GuildID)
			if gs == nil {
				continue
			}

			gs.RLock()
			q.TotalCount = gs.Guild.MemberCount
			gs.RUnlock()
		}
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		leftOver = mu.processing
		logger.WithError(err).Error("failed creating tx")
		return
	}

	// update all the stats
	for _, q := range mu.processing {
		err := mu.setUpdateMemberStatsPeriod(tx, q.GuildID, q.MemberCountMod, q.TotalCount)
		if err != nil {
			leftOver = mu.processing
			tx.Rollback()
			logger.WithError(err).Error("failed updating member stats, rollbacking...")
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("failed comitting updating member results")
		leftOver = mu.processing
		tx.Rollback()
		return
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
