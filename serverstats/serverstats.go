package serverstats

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Server Stats"
}

func RegisterPlugin() {
	common.ValidateSQLSchema(DBSchema)
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		log.WithError(err).Error("serverstats: failed initializing db schema, serverstats will be disabled")
		return
	}

	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
}

var stopStatsLoop = make(chan *sync.WaitGroup)

func UpdateStatsLoop() {

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
		case wg := <-stopStatsLoop:
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

type ChannelStats struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type FullStats struct {
	ChannelsHour map[string]*ChannelStats `json:"channels_hour"`
	JoinedDay    int                      `json:"joined_day"`
	LeftDay      int                      `json:"left_day"`
	Online       int                      `json:"online_now"`
	TotalMembers int                      `json:"total_members_now"`
}

func RetrieveFullStats(guildID int64) (*FullStats, error) {
	// Query the short term stats and the long term stats
	// TODO: If we start moving them over in between we will get somehwat incorrect stats
	// not sure how to fix other than locking

	stats, err := RetrieveRedisStats(guildID)
	if err != nil {
		return nil, err
	}

	rows, err := models.ServerStatsPeriods(qm.Where("guild_id = ?", guildID), qm.Where("started > ?", time.Now().Add(time.Hour*-24))).AllG(context.Background())
	// rows, err := ServerStatsPeriodStore.FindAll(models.NewStatsPeriodQuery().FindByGuildID(
	// 	kallax.Eq, guildID).FindByStarted(kallax.Gt))

	if err != nil {
		return nil, err
	}

	// Merge the stats togheter
	for _, period := range rows {
		stringedChannel := strconv.FormatInt(period.ChannelID.Int64, 10)
		if st, ok := stats.ChannelsHour[stringedChannel]; ok {
			st.Count += period.Count.Int64
		} else {
			stats.ChannelsHour[stringedChannel] = &ChannelStats{
				Name:  stringedChannel,
				Count: period.Count.Int64,
			}
		}
	}

	return stats, nil
}

func RetrieveRedisStats(guildID int64) (*FullStats, error) {
	now := time.Now()
	yesterday := now.Add(time.Hour * -24)
	unixYesterday := discordgo.StrID(yesterday.Unix())
	strGID := discordgo.StrID(guildID)

	var messageStatsRaw []string
	var joined int
	var left int
	var online int64
	var members int

	pipeCmd := radix.Pipeline(
		radix.Cmd(&messageStatsRaw, "ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, unixYesterday, "+inf"),
		radix.Cmd(&joined, "ZCOUNT", "guild_stats_members_joined_day:"+strGID, unixYesterday, "+inf"),
		radix.Cmd(&left, "ZCOUNT", "guild_stats_members_left_day:"+strGID, unixYesterday, "+inf"),
		radix.Cmd(&members, "GET", "guild_stats_num_members:"+strGID),
	)

	err := common.RedisPool.Do(pipeCmd)
	if err != nil {
		return nil, err
	}

	online, err = botrest.GetOnlineCount(guildID)
	if err != nil {
		if botrest.BotIsRunning() {
			log.WithError(err).Error("Failed fetching online count")
		}
	}

	channelResult, err := GetChannelMessageStats(messageStatsRaw, guildID)
	if err != nil {
		return nil, err
	}

	stats := &FullStats{
		ChannelsHour: channelResult,
		JoinedDay:    joined,
		LeftDay:      left,
		Online:       int(online),
		TotalMembers: members,
	}

	return stats, nil
}

func GetChannelMessageStats(raw []string, guildID int64) (map[string]*ChannelStats, error) {

	channelResult := make(map[string]*ChannelStats)
	for _, result := range raw {
		split := strings.Split(result, ":")
		if len(split) < 2 {

			log.WithFields(log.Fields{
				"guild":  guildID,
				"result": result,
			}).Error("Invalid message stats")

			continue
		}
		channelID := split[0]

		stats, ok := channelResult[channelID]
		if ok {
			stats.Count++
		} else {
			name := channelID
			channelResult[channelID] = &ChannelStats{
				Name:  name,
				Count: 1,
			}
		}
	}
	return channelResult, nil
}

// ServerStatsConfig represents a configuration for a server
// reason we dont reference the model directly is because i need to figure out a way to
// migrate them over to the new schema, painlessly.
type ServerStatsConfig struct {
	Public         bool
	IgnoreChannels string

	ParsedChannels []int64
}

func (s *ServerStatsConfig) ParseChannels() {
	split := strings.Split(s.IgnoreChannels, ",")
	for _, v := range split {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			s.ParsedChannels = append(s.ParsedChannels, parsed)
		}
	}
}

func configFromModel(model *models.ServerStatsConfig) *ServerStatsConfig {
	conf := &ServerStatsConfig{
		Public:         model.Public.Bool,
		IgnoreChannels: model.IgnoreChannels.String,
	}
	conf.ParseChannels()

	return conf
}

func GetConfig(ctx context.Context, GuildID int64) (*ServerStatsConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conf, err := models.FindServerStatsConfigG(ctx, GuildID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if conf == nil {
		return &ServerStatsConfig{}, nil
	}

	return configFromModel(conf), nil
}
