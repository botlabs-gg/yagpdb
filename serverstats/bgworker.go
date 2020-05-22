package serverstats

import (
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/premium"
	"github.com/lib/pq"
	"github.com/mediocregopher/radix/v3"
)

const (
	RedisKeyLastHourlyRan = "serverstats_last_hourly_worker_ran"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	go p.updateStatsLoop()
	compressor := &Compressor{}
	go compressor.runLoop(p)

	err := StartMigrationToV2Format()
	if err != nil {
		logger.WithError(err).Error("failed starting migration to v2 format")
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Add(1) // one extra since this is 2 workers

	p.stopStatsLoop <- wg
	p.stopStatsLoop <- wg
}

func (p *Plugin) updateStatsLoop() {

	cleanupTicker := time.NewTicker(time.Hour)

	for {
		select {
		case <-cleanupTicker.C:
			logger.Info("Cleaning up server stats")
			started := time.Now()
			p.cleanupOldStats(time.Now().Add(time.Hour * -30))
			logger.Infof("Took %s to ckean up stats", time.Since(started))
		case wg := <-p.stopStatsLoop:
			wg.Done()
			return
		}
	}
}

type Compressor struct {
	lastCompressed time.Time
}

func (c *Compressor) runLoop(p *Plugin) {
	for {
		_, wait, err := c.updateCompress(time.Now())
		if err != nil {
			wait = 0
			logger.WithError(err).Error("failed compressing stats")
		}
		logger.Info("wait is ", wait)
		after := time.After(wait)

		select {
		case wg := <-p.stopStatsLoop:
			wg.Done()
			logger.Infof("stopped compressor")
			return
		case <-after:
			continue
		}
	}
}

// returns true if a compression was ran
func (c *Compressor) updateCompress(t time.Time) (ran bool, next time.Duration, err error) {
	truncatedDay := t.Truncate(time.Hour * 24)
	if truncatedDay == c.lastCompressed {
		// should not compress
		next = truncatedDay.Add((time.Hour * 25)).Sub(t)
		return false, next, nil
	}

	started := time.Now()

	err = c.RunCompression(truncatedDay)
	if err != nil {
		return true, 0, errors.WithStackIf(err)
	}

	logger.Infof("took %s to compress stats", time.Since(started))

	c.lastCompressed = t

	next = truncatedDay.Add((time.Hour * 25)).Sub(t)
	return true, next, nil
}

func (p *Plugin) getLastTimeRanHourly() time.Time {
	var last int64
	err := common.RedisPool.Do(radix.Cmd(&last, "GET", RedisKeyLastHourlyRan))
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed getting last hourly worker run time")
	}
	return time.Unix(last, 0)
}

func (c *Compressor) RunCompression(t time.Time) error {
	logger.Info("Compressing server stats...")

	// first get a list of active guilds to clean
	activeGuildsMsgs, activeGuildsMisc, err := getActiveGuilds(t)
	if err != nil {
		return errors.WithStackIf(err)
	}

	// process misc stats and message stats combined
	for _, v := range activeGuildsMisc {
		err := compressGuild(t, v, common.ContainsInt64Slice(activeGuildsMsgs, v), true)
		if err != nil {
			return errors.WithStackIf(err)
		}
	}

	// process the message stats only
	for _, v := range activeGuildsMsgs {
		if common.ContainsInt64Slice(activeGuildsMisc, v) {
			continue
		}

		err := compressGuild(t, v, true, false)
		if err != nil {
			return errors.WithStackIf(err)
		}
	}

	return nil
}

func compressGuild(t time.Time, guildID int64, activeMsgs bool, misc bool) error {
	var result []*CompressedStats
	var err error

	// combine both into a compressed per day format
	if misc {
		result, err = compressGuildMiscStats(t, guildID)
		if err != nil {
			return errors.WithStackIf(err)
		}
	}

	if activeMsgs {
		result, err = compressGuildMessageStats(t, guildID, result)
		if err != nil {
			return errors.WithStackIf(err)
		}
	}

	if len(result) < 1 {
		logger.Infof("No stats to update even tough guild was marked active? weeeeird.... %d", guildID)
		return nil
	}

	// commit results into the compressed stats table
	const updateQ = `INSERT INTO server_stats_periods_compressed 
	(guild_id, t, premium, num_messages, num_members, max_online, joins, leaves, max_voice)
	VALUES ($1, $2, $3,      $4,            $5,            $6,       $7,     $8,   $9)
	ON CONFLICT (guild_id, t) DO UPDATE SET
	num_messages = server_stats_periods_compressed.num_messages + $4,
	num_members = GREATEST (server_stats_periods_compressed.num_members, $5),
	max_online = GREATEST (server_stats_periods_compressed.max_online, $6),
	max_voice = GREATEST (server_stats_periods_compressed.max_voice, $9),
	joins = server_stats_periods_compressed.joins + $7,
	leaves = server_stats_periods_compressed.leaves + $8;`

	isPremium, err := premium.IsGuildPremium(guildID)
	if err != nil {
		return errors.WithStackIf(err)
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return errors.WithStackIf(err)
	}

	for _, v := range result {
		_, err = tx.Exec(updateQ, guildID, v.T, isPremium, v.NumMessages, v.TotalMembers, v.MaxOnline, v.Joins, v.Leaves, v.MaxVoice)
		if err != nil {
			tx.Rollback()
			return errors.WithStackIf(err)
		}
	}

	// and finally  delete the uncrompressed stats
	if activeMsgs {
		_, err = tx.Exec("UPDATE server_stats_hourly_periods_messages SET compressed=true WHERE t < $1 AND guild_id = $2;", t, guildID)
		if err != nil {
			tx.Rollback()
			return errors.WithStackIf(err)
		}
	}

	if misc {
		_, err = tx.Exec("UPDATE server_stats_hourly_periods_misc SET compressed=true WHERE t < $1 AND guild_id = $2;", t, guildID)
		if err != nil {
			tx.Rollback()
			return errors.WithStackIf(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.WithStackIf(err)
	}

	return nil
}

type CompressedStats struct {
	GuildID int64
	T       time.Time

	NumMessages  int
	MaxOnline    int
	TotalMembers int
	Joins        int
	Leaves       int
	MaxVoice     int
}

func compressGuildMessageStats(t time.Time, guildID int64, mergeWith []*CompressedStats) ([]*CompressedStats, error) {

	const q = `SELECT t, count 
	FROM server_stats_hourly_periods_messages
	WHERE t < $1 AND guild_id = $2 AND compressed=false;
	`

	rows, err := common.PQ.Query(q, t, guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

OUTER:
	for rows.Next() {
		var t time.Time
		var count int

		err = rows.Scan(&t, &count)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		roundedToDay := t.Truncate(time.Hour * 24)

		for _, r := range mergeWith {
			if r.T != roundedToDay {
				continue
			}

			// found existing compressed day result, merge this with that one
			r.NumMessages += count
			continue OUTER
		}

		// no existing compressed day result, make a new one
		mergeWith = append(mergeWith, &CompressedStats{
			GuildID:     guildID,
			T:           roundedToDay,
			NumMessages: count,
		})
	}

	return mergeWith, nil
}

func compressGuildMiscStats(t time.Time, guildID int64) ([]*CompressedStats, error) {

	const q = `SELECT t, num_members, max_online, joins, leaves, max_voice 
	FROM server_stats_hourly_periods_misc
	WHERE t < $1 AND guild_id = $2 AND compressed=false;
	`

	rows, err := common.PQ.Query(q, t, guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	result := make([]*CompressedStats, 0)

OUTER:
	for rows.Next() {
		var t time.Time
		var numMembers, maxOnline, joins, leaves, maxVoice int

		err = rows.Scan(&t, &numMembers, &maxOnline, &joins, &leaves, &maxVoice)
		if err != nil {
			return nil, errors.WithStackIf(err)
		}

		roundedToDay := t.Truncate(time.Hour * 24)

		for _, r := range result {
			if r.T != roundedToDay {
				continue
			}

			// found existing compressed day result, merge this with that one
			r.Joins += joins
			r.Leaves += leaves

			if r.TotalMembers < numMembers {
				r.TotalMembers = numMembers
			}
			if r.MaxOnline < maxOnline {
				r.MaxOnline = maxOnline
			}
			if r.MaxVoice < maxVoice {
				r.MaxVoice = maxVoice
			}
			continue OUTER
		}

		// no existing compressed day result, make a new one
		result = append(result, &CompressedStats{
			GuildID:      guildID,
			T:            roundedToDay,
			TotalMembers: numMembers,
			MaxOnline:    maxOnline,
			Joins:        joins,
			Leaves:       leaves,
			MaxVoice:     maxVoice,
		})
	}

	return result, nil
}

func getActiveGuilds(t time.Time) (activeMsgs []int64, activeMisc []int64, err error) {
	const qMsgs = `SELECT DISTINCT guild_id FROM server_stats_hourly_periods_messages WHERE t < $1 AND compressed=false`
	rows, err := common.PQ.Query(qMsgs, t)
	if err != nil {
		return nil, nil, errors.WithStackIf(err)
	}

	for rows.Next() {
		var g int64
		err = rows.Scan(&g)
		if err != nil {
			rows.Close()
			return nil, nil, errors.WithStackIf(err)
		}

		activeMsgs = append(activeMsgs, g)
	}
	rows.Close()

	const qMisc = `SELECT DISTINCT guild_id FROM server_stats_hourly_periods_misc WHERE t < $1 AND compressed=false`
	rows, err = common.PQ.Query(qMisc, t)
	if err != nil {
		return nil, nil, errors.WithStackIf(err)
	}

	for rows.Next() {
		var g int64
		err = rows.Scan(&g)
		if err != nil {
			rows.Close()
			return nil, nil, errors.WithStackIf(err)
		}

		activeMisc = append(activeMisc, g)
	}
	rows.Close()

	return
}

func (p *Plugin) cleanupOldStats(t time.Time) error {
	// and finally delete the already compressed stats
	_, err := common.PQ.Exec("DELETE FROM server_stats_hourly_periods_messages WHERE t < $1 AND compressed = true;", t)
	if err != nil {
		return errors.WithStackIf(err)
	}

	_, err = common.PQ.Exec("DELETE FROM server_stats_hourly_periods_misc WHERE t < $1 AND compressed = true;", t)
	if err != nil {
		return errors.WithStackIf(err)
	}

	return nil
}

func (p *Plugin) RunCompressionLegacy() {
	premiumServers, err := premium.AllGuildsOncePremium()
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed retrieving premium guilds")
		return
	}

	// convert it to a slice
	premiumSlice := make([]int64, 0, len(premiumServers))
	for k, _ := range premiumServers {
		premiumSlice = append(premiumSlice, k)
	}

	numDelete := int64(0)

	started := time.Now()
	del, err := common.PQ.Exec("DELETE FROM server_stats_periods WHERE started < NOW() - INTERVAL '7 days' AND not (guild_id = ANY ($1))", pq.Int64Array(premiumSlice))
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed deleting old message stats")
	} else if del != nil {
		affected, _ := del.RowsAffected()
		numDelete += affected
	}

	del, err = common.PQ.Exec("DELETE FROM server_stats_member_periods WHERE created_at < NOW() - INTERVAL '7 days' AND not (guild_id = ANY ($1))", pq.Int64Array(premiumSlice))
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed deleting old member stats")
	} else if del != nil {
		affected, _ := del.RowsAffected()
		numDelete += affected
	}

	logger.Infof("[serverstats] Deleted %d records in %s", numDelete, time.Since(started))

	secondRunStarted := time.Now()
	tx, err := common.PQ.Begin()
	if err != nil {
		logger.WithError(err).Error("failed starting transaction")
		return
	}

	totalDeleted := int64(0)
	for g, v := range premiumServers {
		if time.Since(v) < time.Hour*48 {
			continue
		}
		result, err := tx.Exec("DELETE FROM server_stats_periods WHERE guild_id = $1 AND started > $2  AND NOW() - INTERVAL '7 days'  > started", g, v)
		if err != nil {
			logger.WithError(err).WithField("guild", g).Error("[serverstats] failed running cleanup query on premium guild message stats")
			tx.Rollback()
			return
		}

		affected, _ := result.RowsAffected()
		totalDeleted += affected

		result, err = tx.Exec("DELETE FROM server_stats_member_periods WHERE guild_id = $1 AND created_at > $2  AND NOW() - INTERVAL '7 days'  > created_at", g, v)
		if err != nil {
			logger.WithError(err).WithField("guild", g).Error("[serverstats] failed running cleanup query on premium guild member stats")
			tx.Rollback()
			return
		}

		affected, _ = result.RowsAffected()
		totalDeleted += affected
	}

	err = tx.Commit()
	if err != nil {
		logger.WithError(err).Error("[serverstats] cleanup failed comitting transaction")
		return
	}

	logger.Infof("[serverstats] slow premium specific cleanup took %s, deleted %d records (num premium %d)", time.Since(secondRunStarted), totalDeleted, len(premiumServers))
}

func getActiveServersList(key string, full bool) ([]int64, error) {
	var guilds []int64
	if full {
		err := common.RedisPool.Do(radix.Cmd(&guilds, "SMEMBERS", "connected_guilds"))
		return guilds, errors.WrapIf(err, "smembers conn_guilds")
	}

	var exists bool
	if common.LogIgnoreError(common.RedisPool.Do(radix.Cmd(&exists, "EXISTS", key)), "[serverstats] "+key, nil); !exists {
		return guilds, nil // no guilds to process
	}

	err := common.RedisPool.Do(radix.Cmd(nil, "RENAME", key, key+"_processing"))
	if err != nil {
		return guilds, errors.WrapIf(err, "rename")
	}

	err = common.RedisPool.Do(radix.Cmd(&guilds, "SMEMBERS", key+"_processing"))
	if err != nil {
		return guilds, errors.WrapIf(err, "smembers")
	}

	common.LogIgnoreError(common.RedisPool.Do(radix.Cmd(nil, "DEL", "serverstats_active_guilds_processing")), "[serverstats] del "+key, nil)
	return guilds, err
}
