package serverstats

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/backgroundworkers"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"strconv"
	"strings"
	"sync"
	"time"
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
	UpdateOldStats()

	tempTicker := time.NewTicker(time.Minute)
	longTicker := time.NewTicker(time.Hour)

	for {
		select {
		case <-tempTicker.C:
			ProcessTempStats(false)
		case <-longTicker.C:
			go UpdateOldStats()
		case wg := <-p.stopStatsLoop:
			wg.Done()
			return
		}
	}
}

// ProcessTempStats moves stats from redis to postgres batched up
func ProcessTempStats(full bool) {

	started := time.Now()

	var strGuilds []string
	if full {
		err := common.RedisPool.Do(radix.Cmd(&strGuilds, "SMEMBERS", "connected_guilds"))
		if err != nil {
			log.WithError(err).Error("Failed retrieving connected guilds")
		}
	} else {
		var exists bool
		if common.RedisPool.Do(radix.Cmd(&exists, "EXISTS", "serverstats_active_guilds")); !exists {
			return // no guilds to process
		}

		err := common.RedisPool.Do(radix.Cmd(nil, "RENAME", "serverstats_active_guilds", "serverstats_active_guilds_processing"))
		if err != nil {
			log.WithError(err).Error("Failed renaming temp stats")
			return
		}

		err = common.RedisPool.Do(radix.Cmd(&strGuilds, "SMEMBERS", "serverstats_active_guilds_processing"))
		if err != nil {
			log.WithError(err).Error("Failed retrieving active guilds")
		}

		common.RedisPool.Do(radix.Cmd(nil, "DEL", "serverstats_active_guilds_processing"))
	}

	if len(strGuilds) < 1 {
		log.Info("Skipped updating stats, no activity")
		return
	}

	for _, strGID := range strGuilds {
		g, _ := strconv.ParseInt(strGID, 10, 64)

		err := UpdateGuildStats(g)
		if err != nil {
			log.WithFields(log.Fields{
				"guild":      g,
				log.ErrorKey: err,
			}).Error("Failed updating stats")
		}
	}

	log.WithFields(log.Fields{
		"duration":    time.Since(started).Seconds(),
		"num_servers": len(strGuilds),
	}).Info("Updated temp stats")
}

// Updates the stats on a specific guild, removing expired stats
func UpdateGuildStats(guildID int64) error {
	now := time.Now()
	minAgo := now.Add(time.Minute)
	unixminAgo := minAgo.Unix()

	yesterday := now.Add(24 * -time.Hour)
	unixYesterday := yesterday.Unix()

	cmds := make([]radix.CmdAction, 4)

	strGID := discordgo.StrID(guildID)

	var messageStatsRaw []string
	cmds[0] = radix.FlatCmd(&messageStatsRaw, "ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo)
	cmds[1] = radix.FlatCmd(nil, "ZREMRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo)
	cmds[2] = radix.FlatCmd(nil, "ZREMRANGEBYSCORE", "guild_stats_members_joined_day:"+strGID, "-inf", unixYesterday)
	cmds[3] = radix.FlatCmd(nil, "ZREMRANGEBYSCORE", "guild_stats_members_left_day:"+strGID, "-inf", unixYesterday)

	err := common.RedisPool.Do(radix.Pipeline(cmds...))
	if err != nil {
		return err
	}

	channelAuthorStats := make(map[string]*models.ServerStatsPeriod)
	for _, row := range messageStatsRaw {
		// 0 = channel, 1 = mid, 2 = author
		split := strings.Split(row, ":")

		if len(split) < 2 {
			log.WithField("guild", guildID).Error("Invalid stats entry, skipping")
			continue
		}

		channel := split[0]
		author := split[2]

		if model, ok := channelAuthorStats[channel+"_"+author]; ok {
			model.Count.Int64++
		} else {
			model = &models.ServerStatsPeriod{
				GuildID:   null.Int64From(guildID),
				ChannelID: null.Int64From(common.MustParseInt(channel)),
				UserID:    null.Int64From(common.MustParseInt(author)),
				Started:   null.TimeFrom(minAgo), // TODO: we should calculate these from the min max snowflake ids
				Duration:  null.Int64From(int64(time.Minute)),
				Count:     null.Int64From(1),
			}
			channelAuthorStats[channel+"_"+author] = model
		}
	}

	tx, err := common.PQ.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.WithMessage(err, "bginTX")
	}

	for _, model := range channelAuthorStats {
		err = model.Insert(context.Background(), tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return errors.WithMessage(err, "insert")
		}
	}

	err = tx.Commit()
	err = errors.WithMessage(err, "commit")
	return err
}

func UpdateOldStats() {
	started := time.Now()
	del, err := common.PQ.Exec("DELETE FROM server_stats_periods WHERE started < NOW() - INTERVAL '2 days'")
	if err != nil {
		log.WithError(err).Error("ServerStats: Failed deleting old stats")
	} else if del != nil {
		affected, _ := del.RowsAffected()
		log.Infof("ServerStats: Deleted %d records in %s", affected, time.Since(started))
	}
}
