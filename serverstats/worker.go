package serverstats

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"strings"
	"sync"
	"time"
)

const (
	RedisKeyLastHourlyRan = "serverstats_last_hourly_worker_ran"
)

var _ common.BackgroundWorkerPlugin = (*Plugin)(nil)

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
	err := common.RedisPool.Do(radix.Cmd(&last, "GET", RedisKeyLastHourlyRan))
	if err != nil {
		log.WithError(err).Error("[serverstats] failed getting last hourly worker run time")
	}
	return time.Unix(last, 0)
}

// ProcessTempStats moves stats from redis to postgres batched up
func ProcessTempStats(full bool) {

	started := time.Now()

	// first retrieve all the guilds that should be processed
	activeGuilds, err := getActiveServersList("serverstats_active_guilds", full)
	if err != nil {
		log.WithError(err).Error("[serverstats] failed retrieving active guilds")
		return
	}

	if len(activeGuilds) < 1 {
		log.Info("[serverstats] skipped moving temp message stats to postgres, no activity")
		return // no guilds to process
	}

	for _, g := range activeGuilds {
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
		"num_servers": len(activeGuilds),
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

	channelStats := make(map[string]*models.ServerStatsPeriod)
	for _, row := range messageStatsRaw {
		// 0 = channel, 1 = mid, 2 = author
		split := strings.Split(row, ":")

		if len(split) < 2 {
			log.WithField("guild", guildID).Error("Invalid stats entry, skipping")
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
	err = errors.WithMessage(err, "commit")
	return err
}

// func (p *Plugin) RunHourlyTasks(full bool) {
// 	// guildMembersChangedServers :=

// 	// first retrieve all the guilds that should be processed
// 	guildMembersChangedServers, err := getActiveServersList(RedisKeyGuildMembersChanged, full)
// 	if err != nil {
// 		log.WithError(err).Error("[serverstats] failed retrieving active guilds")
// 		return
// 	}

// 	if len(guildMembersChangedServers) < 1 {
// 		log.Info("[serverstats] skipped updating changed members, no activity")
// 		return // no guilds to process
// 	}

// 	lastRunTime := p.getLastTimeRanHourly()

// 	// we do minus 1 second to avoid missing certain events in the current second
// 	now := time.Now().Add(-time.Second).Unix()
// 	for _, v := range guildMembersChangedServers {
// 		p.RunHourlyGuildMemberTasks(v, lastRunTime, now)
// 	}

// 	common.LogIgnoreError(common.RedisPool.Do(radix.FlatCmd(nil, "SET", RedisKeyLastHourlyRan, now)), "[serverstats] failed setting last time ran", nil)
// }

// func (p *Plugin) RunHourlyGuildMemberTasks(guildID int64, lastRunTime time.Time, now int64) {
// 	var totalMemberCount int64
// 	var joins int64
// 	var leaves int64

// 	lastRunUnix := lastRunTime.Unix() + 1
// 	common.LogIgnoreError(common.RedisPool.Do(radix.Cmd(&totalMemberCount, "GET", RedisKeyTotalMembers(guildID))),
// 		"[serverstats] get total members", nil)

// 	common.LogIgnoreError(common.RedisPool.Do(radix.FlatCmd(&joins, "ZCOUNT", RedisKeyMembersJoined(guildID), lastRunUnix, now)),
// 		"[serverstats] get joined", nil)

// 	common.LogIgnoreError(common.RedisPool.Do(radix.FlatCmd(&leaves, "ZCOUNT", RedisKeyMembersLeft(guildID), lastRunUnix, now)),
// 		"[serverstats] get left", nil)

// 	// radix.Cmd(&members, "GET", "guild_stats_num_members:"+strGID),

// 	model := &models.ServerStatsMemberPeriod{
// 		GuildID:    guildID,
// 		Joins:      joins,
// 		Leaves:     leaves,
// 		NumMembers: totalMemberCount,
// 	}

// 	err := model.InsertG(context.Background(), boil.Infer())
// 	if err != nil {
// 		log.WithError(err).Error("[serverstats] failed inserting member stats period")
// 	}
// }

func (p *Plugin) RunCleanup() {
	started := time.Now()
	del, err := common.PQ.Exec("DELETE FROM server_stats_periods WHERE started < NOW() - INTERVAL '2 days'")
	if err != nil {
		log.WithError(err).Error("ServerStats: Failed deleting old stats")
	} else if del != nil {
		affected, _ := del.RowsAffected()
		log.Infof("ServerStats: Deleted %d records in %s", affected, time.Since(started))
	}
}

func getActiveServersList(key string, full bool) ([]int64, error) {
	var guilds []int64
	if full {
		err := common.RedisPool.Do(radix.Cmd(&guilds, "SMEMBERS", "connected_guilds"))
		return guilds, errors.Wrap(err, "smembers conn_guilds")
	}

	var exists bool
	if common.LogIgnoreError(common.RedisPool.Do(radix.Cmd(&exists, "EXISTS", key)), "[serverstats] "+key, nil); !exists {
		return guilds, nil // no guilds to process
	}

	err := common.RedisPool.Do(radix.Cmd(nil, "RENAME", key, key+"_processing"))
	if err != nil {
		return guilds, errors.Wrap(err, "rename")
	}

	err = common.RedisPool.Do(radix.Cmd(&guilds, "SMEMBERS", key+"_processing"))
	if err != nil {
		return guilds, errors.Wrap(err, "smembers")
	}

	common.LogIgnoreError(common.RedisPool.Do(radix.Cmd(nil, "DEL", "serverstats_active_guilds_processing")), "[serverstats] del "+key, nil)
	return guilds, err
}
