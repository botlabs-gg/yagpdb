package serverstats

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

const (
	RedisKeyLastHourlyRan = "serverstats_last_hourly_worker_ran"
)

var _ backgroundworkers.BackgroundWorkerPlugin = (*Plugin)(nil)

func (p *Plugin) RunBackgroundWorker() {
	p.UpdateStatsLoop()
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	p.stopStatsLoop <- wg
}

func (p *Plugin) UpdateStatsLoop() {

	ProcessTempStats(true)
	p.RunCleanup()

	tempTicker := time.NewTicker(time.Minute)
	cleanupTicker := time.NewTicker(time.Hour)

	for {
		select {
		case <-tempTicker.C:
			ProcessTempStats(false)
		case <-cleanupTicker.C:
			go p.RunCleanup()
		case wg := <-p.stopStatsLoop:
			wg.Done()
			return
		}
	}
}

func (p *Plugin) getLastTimeRanHourly() time.Time {
	var last int64
	err := common.RedisPool.Do(retryableredis.Cmd(&last, "GET", RedisKeyLastHourlyRan))
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed getting last hourly worker run time")
	}
	return time.Unix(last, 0)
}

// ProcessTempStats moves stats from redis to postgres batched up
func ProcessTempStats(full bool) {

	started := time.Now()

	// first retrieve all the guilds that should be processed
	activeGuilds, err := getActiveServersList("serverstats_active_guilds", full)
	if err != nil {
		logger.WithError(err).Error("[serverstats] failed retrieving active guilds")
		return
	}

	if len(activeGuilds) < 1 {
		logger.Info("[serverstats] skipped moving temp message stats to postgres, no activity")
		return // no guilds to process
	}

	for _, g := range activeGuilds {
		err := UpdateGuildStats(g)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"guild":         g,
				logrus.ErrorKey: err,
			}).Error("[serverstats] Failed updating stats")
		}
	}

	logger.WithFields(logrus.Fields{
		"duration":    time.Since(started).Seconds(),
		"num_servers": len(activeGuilds),
	}).Info("[serverstats] Updated temp stats")
}

// Updates the stats on a specific guild, removing expired stats
func UpdateGuildStats(guildID int64) error {
	now := time.Now()
	minAgo := now.Add(-time.Minute)
	unixminAgo := minAgo.Unix()

	strGID := discordgo.StrID(guildID)
	var messageStatsRaw []string

	err := common.RedisPool.Do(retryableredis.FlatCmd(&messageStatsRaw, "ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo))
	if err != nil {
		return err
	}

	channelStats := make(map[string]*models.ServerStatsPeriod)
	for _, row := range messageStatsRaw {
		// 0 = channel, 1 = mid, 2 = author
		split := strings.Split(row, ":")

		if len(split) < 2 {
			logger.WithField("guild", guildID).Error("Invalid stats entry, skipping")
			continue
		}

		channel := split[0]
		// author := split[2]

		if model, ok := channelStats[channel]; ok {
			model.Count.Int64++
		} else {
			model = &models.ServerStatsPeriod{
				GuildID:   null.Int64From(guildID),
				ChannelID: null.Int64From(common.MustParseInt(channel)),
				Started:   null.TimeFrom(minAgo), // TODO: we should calculate these from the min max snowflake ids
				Duration:  null.Int64From(int64(time.Minute)),
				Count:     null.Int64From(1),
			}
			channelStats[channel] = model
		}
	}

	tx, err := common.PQ.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.WithMessage(err, "bginTX")
	}

	for _, model := range channelStats {
		err = model.Insert(context.Background(), tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return errors.WithMessage(err, "insert")
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.WithMessage(err, "commit")
	}

	err = common.RedisPool.Do(retryableredis.FlatCmd(nil, "ZREMRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo))
	if err != nil {
		return errors.WithMessage(err, "zremrangebyscore")
	}

	return err
}

func (p *Plugin) RunCleanup() {
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
		err := common.RedisPool.Do(retryableredis.Cmd(&guilds, "SMEMBERS", "connected_guilds"))
		return guilds, errors.Wrap(err, "smembers conn_guilds")
	}

	var exists bool
	if common.LogIgnoreError(common.RedisPool.Do(retryableredis.Cmd(&exists, "EXISTS", key)), "[serverstats] "+key, nil); !exists {
		return guilds, nil // no guilds to process
	}

	err := common.RedisPool.Do(retryableredis.Cmd(nil, "RENAME", key, key+"_processing"))
	if err != nil {
		return guilds, errors.Wrap(err, "rename")
	}

	err = common.RedisPool.Do(retryableredis.Cmd(&guilds, "SMEMBERS", key+"_processing"))
	if err != nil {
		return guilds, errors.Wrap(err, "smembers")
	}

	common.LogIgnoreError(common.RedisPool.Do(retryableredis.Cmd(nil, "DEL", "serverstats_active_guilds_processing")), "[serverstats] del "+key, nil)
	return guilds, err
}
