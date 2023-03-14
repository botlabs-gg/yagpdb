package serverstats

import (
	"fmt"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/serverstats/messagestatscollector"
	"github.com/mediocregopher/radix/v3"
)

const (
	RedisKeyLastHourlyRan = "serverstats_last_hourly_worker_ran"
)

var (
	confDisableCompression    = config.RegisterOption("yagpdb.serverstats.disable_compression", "Disables compression of serverstats", false)
	confDisableNewCompression = config.RegisterOption("yagpdb.serverstats.disable_new_compression", "Disables compression of serverstats", false)
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	compressorLegacy := &Compressor{}
	go compressorLegacy.runLoopLegacy(p)

	compressorNew := &Compressor{}
	go compressorNew.runLoop(p)

	err := StartMigrationToV2Format()
	if err != nil {
		logger.WithError(err).Error("failed starting migration to v2 format")
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	wg.Add(1)
	p.stopStatsLoop <- wg
	p.stopStatsLoop <- wg
}

type Compressor struct {
	lastCompressed time.Time
}

func (c *Compressor) runLoop(p *Plugin) {
	time.Sleep(time.Second)

	for {
		// find the next time we should run a compression
		_, wait, err := c.updateCompress(time.Now(), false)
		if err != nil {
			wait = time.Second
			logger.WithError(err).Errorf("failed compressing stats: %+v", err)
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

func (c *Compressor) runLoopLegacy(p *Plugin) {
	cleanupTicker := time.NewTicker(time.Hour * 6)
	time.Sleep(time.Second)

	for {
		// find the next time we should run a compression
		_, wait, err := c.updateCompress(time.Now(), true)
		if err != nil {
			wait = time.Second
			logger.WithError(err).Errorf("failed compressing stats: %+v", err)
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
		case <-cleanupTicker.C:
			// run cleanup of temporary stats
			logger.Info("Cleaning up server stats")
			started := time.Now()
			p.cleanupOldStats(time.Now().Add(time.Hour * -30))
			logger.Infof("Took %s to ckean up stats", time.Since(started))
		}
	}
}

// returns true if a compression was ran
func (c *Compressor) updateCompress(t time.Time, legacy bool) (ran bool, next time.Duration, err error) {
	truncatedDay := t.Truncate(time.Hour * 24)
	if truncatedDay == c.lastCompressed {
		// should not compress
		next = truncatedDay.Add((time.Hour * 25)).Sub(t)
		return false, next, nil
	}

	started := time.Now()

	if legacy {
		if !confDisableCompression.GetBool() {
			err = c.RunCompressionLegacy(truncatedDay)
		}
	} else {
		if !confDisableNewCompression.GetBool() {
			err = c.runCompression(t.AddDate(0, 0, -1))
		}
	}

	if err != nil {
		return true, 0, errors.WithStackIf(err)
	}

	if legacy {
		logger.Infof("took %s to compress LEGACY stats", time.Since(started))
	} else {
		logger.Infof("took %s to compress NEW stats", time.Since(started))
	}

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

func (c *Compressor) RunCompressionLegacy(t time.Time) error {
	logger.Info("Compressing server stats...")

	// first get a list of active guilds to clean
	activeGuildsMsgs, activeGuildsMisc, err := getActiveGuilds(t)
	if err != nil {
		return errors.WithStackIf(err)
	}

	// process misc stats and message stats combined
	for _, v := range activeGuildsMisc {
		err := compressGuildLegacy(t, v, common.ContainsInt64Slice(activeGuildsMsgs, v), true)
		if err != nil {
			return errors.WithStackIf(err)
		}

		time.Sleep(time.Millisecond * 10)
	}

	// process the message stats only
	for _, v := range activeGuildsMsgs {
		if common.ContainsInt64Slice(activeGuildsMisc, v) {
			continue
		}

		err := compressGuildLegacy(t, v, true, false)
		if err != nil {
			return errors.WithStackIf(err)
		}

		time.Sleep(time.Millisecond * 10)
	}

	return nil
}

func compressGuildLegacy(t time.Time, guildID int64, activeMsgs bool, misc bool) error {
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

func (c *Compressor) runCompression(t time.Time) error {
	// check up to 5 days back
	logger.Infof("Running compression, t is %d:%d: %s", t.Year(), t.YearDay(), t)
	for i := 0; i < 5; i++ {
		newT := t.AddDate(0, 0, -i)
		year := newT.Year()
		day := newT.YearDay()

		err := c.runCompressionDay(year, day)
		if err != nil {
			return err
		}
	}

	return nil
}

type GuildStatsFrame struct {
	Messages      int
	TotalMembers  int
	OnlineMembers int
	Joins         int
	Leaves        int
}

const keyCompressionCompressionRanDays = "serverstats_compression_days_ran"

func (c *Compressor) hasCompressionRan(year, day int) (bool, error) {
	var ran bool
	err := common.RedisPool.Do(radix.Cmd(&ran, "SISMEMBER", keyCompressionCompressionRanDays, fmt.Sprintf("%d:%d", year, day)))
	return ran, err
}

func (c *Compressor) runCompressionDay(year, day int) error {
	logger.Infof("Running compression for %d:%d", year, day)

	alreadyRan, err := c.hasCompressionRan(year, day)
	if err != nil {
		return err
	}

	if alreadyRan {
		logger.Infof("Stats compression already ran for %d: %d", year, day)
		return c.cleanTempRedisStats(year, day)
	}

	stats, err := c.collectStats(year, day)
	if err != nil {
		return errors.WithStackIf(err)
	}

	if len(stats) > 0 {
		err = c.saveCollectedStats(year, day, stats)
		if err != nil {
			return err
		}
	} else {
		logger.Infof("No stats for %d:%d", year, day)
	}

	return c.cleanTempRedisStats(year, day)
}

func (c *Compressor) cleanTempRedisStats(year, day int) error {
	// clean message stats
	var activeGuilds []int64
	err := common.RedisPool.Do(radix.Cmd(&activeGuilds, "SMEMBERS", messagestatscollector.KeyActiveGuilds(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}

	for _, g := range activeGuilds {
		err = common.RedisPool.Do(radix.Cmd(nil, "DEL", messagestatscollector.KeyMessageStats(g, year, day)))
		if err != nil {
			return errors.WithStackIf(err)
		}
	}

	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", messagestatscollector.KeyActiveGuilds(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}

	// clean other stats
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", keyTotalMembers(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", keyOnlineMembers(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", keyJoinedMembers(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", keyLeftMembers(year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}

	// finally, clean up this state of this day
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", keyCompressionCompressionRanDays, fmt.Sprintf("%d:%d", year, day)))
	if err != nil {
		return errors.WithStackIf(err)
	}

	return err
}

func (c *Compressor) saveCollectedStats(year, day int, stats map[int64]*GuildStatsFrame) error {
	t := time.Date(year, 1, day, 0, 0, 0, 0, time.UTC)

	allPremiumGuilds, err := premium.AllGuildsOncePremium()
	if err != nil {
		return err
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return err
	}

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

	for g, s := range stats {
		_, isPremium := allPremiumGuilds[g]

		_, err = tx.Exec(updateQ, g, t, isPremium, s.Messages, s.TotalMembers, s.OnlineMembers, s.Joins, s.Leaves, 0)
		if err != nil {
			tx.Rollback()
			return errors.WithStackIf(err)
		}
	}

	// mark this day as compressed
	err = common.RedisPool.Do(radix.Cmd(nil, "SADD", keyCompressionCompressionRanDays, fmt.Sprintf("%d:%d", year, day)))
	if err != nil {
		tx.Rollback()
		return errors.WithStackIf(err)
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()

		// try to rollback marking this guild as compressed
		err2 := common.RedisPool.Do(radix.Cmd(nil, "SREM", keyCompressionCompressionRanDays, fmt.Sprintf("%d:%d", year, day)))
		if err2 != nil {
			// this requires manual internvention to repair, broken connection to db or something in the middle of commit?
			// but atleast this wont produce duplicate stats
			logger.WithError(err2).Error("FAILED UN-MARKING GUILD AS COMPRESSED")
		}

		return errors.WithStackIf(err)
	}

	return nil
}

func (c *Compressor) collectStats(year, day int) (map[int64]*GuildStatsFrame, error) {
	stats, err := c.collectMessageStats(year, day)
	if err != nil {
		return stats, errors.WithStackIf(err)
	}

	err = c.collectTotalMembers(year, day, stats)
	if err != nil {
		return stats, errors.WithStackIf(err)
	}

	err = c.collectOnlineMembers(year, day, stats)
	if err != nil {
		return stats, errors.WithStackIf(err)
	}

	err = c.collectJoins(year, day, stats)
	if err != nil {
		return stats, errors.WithStackIf(err)
	}

	err = c.collectLeaves(year, day, stats)
	if err != nil {
		return stats, errors.WithStackIf(err)
	}

	return stats, nil
}

func (c *Compressor) collectMessageStats(year, day int) (map[int64]*GuildStatsFrame, error) {
	var activeGuilds []int64
	err := common.RedisPool.Do(radix.Cmd(&activeGuilds, "SMEMBERS", messagestatscollector.KeyActiveGuilds(year, day)))
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*GuildStatsFrame)
	if len(activeGuilds) < 1 {
		return result, nil
	}

	for _, g := range activeGuilds {
		raw := make(map[int64]int64)
		err = common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", messagestatscollector.KeyMessageStats(g, year, day), "0", "-1", "WITHSCORES"))
		if err != nil {
			return nil, err
		}

		combined := 0
		for _, v := range raw {
			combined += int(v)
		}

		result[g] = &GuildStatsFrame{
			Messages: combined,
		}
	}

	return result, nil
}

func (c *Compressor) collectTotalMembers(year, day int, stats map[int64]*GuildStatsFrame) error {
	raw := make(map[int64]int64)
	err := common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", keyTotalMembers(year, day), "0", "-1", "WITHSCORES"))
	if err != nil {
		return err
	}

	for g, v := range raw {
		if current, ok := stats[g]; ok {
			current.TotalMembers += int(v)
		} else {
			stats[g] = &GuildStatsFrame{
				TotalMembers: int(v),
			}
		}
	}

	return nil
}

func (c *Compressor) collectJoins(year, day int, stats map[int64]*GuildStatsFrame) error {
	raw := make(map[int64]int64)
	err := common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", keyJoinedMembers(year, day), "0", "-1", "WITHSCORES"))
	if err != nil {
		return err
	}

	for g, v := range raw {
		if current, ok := stats[g]; ok {
			current.Joins += int(v)
		} else {
			stats[g] = &GuildStatsFrame{
				Joins: int(v),
			}
		}
	}

	return nil
}

func (c *Compressor) collectLeaves(year, day int, stats map[int64]*GuildStatsFrame) error {
	raw := make(map[int64]int64)
	err := common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", keyLeftMembers(year, day), "0", "-1", "WITHSCORES"))
	if err != nil {
		return err
	}

	for g, v := range raw {
		if current, ok := stats[g]; ok {
			current.Leaves += int(v)
		} else {
			stats[g] = &GuildStatsFrame{
				Leaves: int(v),
			}
		}
	}

	return nil
}

func (c *Compressor) collectOnlineMembers(year, day int, stats map[int64]*GuildStatsFrame) error {
	raw := make(map[int64]int64)
	err := common.RedisPool.Do(radix.Cmd(&raw, "ZRANGE", keyOnlineMembers(year, day), "0", "-1", "WITHSCORES"))
	if err != nil {
		return err
	}

	for g, v := range raw {
		if current, ok := stats[g]; ok {
			current.OnlineMembers += int(v)
		} else {
			stats[g] = &GuildStatsFrame{
				OnlineMembers: int(v),
			}
		}
	}

	return nil
}
