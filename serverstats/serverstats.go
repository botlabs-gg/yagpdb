package serverstats

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/mediocregopher/radix"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Plugin struct {
	stopStatsLoop chan *sync.WaitGroup
}

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

	plugin := &Plugin{
		stopStatsLoop: make(chan *sync.WaitGroup),
	}
	common.RegisterPlugin(plugin)
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
