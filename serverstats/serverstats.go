package serverstats

import (
	"context"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/serverstats/models"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-kallax.v1"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ServerStatsPeriodStore *models.StatsPeriodStore
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Server Stats"
}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)

	common.GORM.AutoMigrate(&ServerStatsConfig{})
	configstore.RegisterConfig(configstore.SQL, &ServerStatsConfig{})
	ServerStatsPeriodStore = models.NewStatsPeriodStore(common.PQ)

	common.ValidateSQLSchema(DBSchema)
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		panic("Failed initializing db schema: " + err.Error())
	}
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
	client := common.MustGetRedisClient()
	defer common.RedisPool.Put(client)

	started := time.Now()

	var strGuilds []string
	if full {
		var err error
		strGuilds, err = client.Cmd("SMEMBERS", "connected_guilds").List()
		if err != nil {
			log.WithError(err).Error("Failed retrieving connected guilds")
		}
	} else {
		err := client.Cmd("RENAME", "serverstats_active_guilds", "serverstats_active_guilds_processing").Err
		if err != nil {
			log.WithError(err).Error("Failed renaming temp stats")
			return
		}

		strGuilds, err = client.Cmd("SMEMBERS", "serverstats_active_guilds_processing").List()
		if err != nil {
			log.WithError(err).Error("Failed retrieving active guilds")
		}

		client.Cmd("DEL", "serverstats_active_guilds_processing")
	}

	if len(strGuilds) < 1 {
		log.Info("Skipped updating stats, no activity")
		return
	}

	for _, strGID := range strGuilds {
		g, _ := strconv.ParseInt(strGID, 10, 64)

		err := UpdateGuildStats(client, g)
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
func UpdateGuildStats(client *redis.Client, guildID int64) error {
	now := time.Now()
	minAgo := now.Add(time.Minute)
	unixminAgo := minAgo.Unix()

	yesterday := now.Add(24 * -time.Hour)
	unixYesterday := yesterday.Unix()

	strGID := discordgo.StrID(guildID)
	client.PipeAppend("ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo)
	client.PipeAppend("ZREMRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, "-inf", unixminAgo)
	client.PipeAppend("ZREMRANGEBYSCORE", "guild_stats_members_joined_day:"+strGID, "-inf", unixYesterday)
	client.PipeAppend("ZREMRANGEBYSCORE", "guild_stats_members_left_day:"+strGID, "-inf", unixYesterday)

	replies, err := common.GetRedisReplies(client, 4)
	if err != nil {
		return err
	}

	messageStatsRaw, err := replies[0].List()
	if err != nil {
		return err
	}

	channelAuthorStats := make(map[string]*models.StatsPeriod)
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
			model.Count++
		} else {
			model = &models.StatsPeriod{
				GuildID:   guildID,
				ChannelID: common.MustParseInt(channel),
				UserID:    common.MustParseInt(author),
				Started:   minAgo, // TODO: we should calculate these from the min max snowflake ids
				Duration:  time.Minute,
				Count:     1,
			}
			channelAuthorStats[channel+"_"+author] = model
		}
	}

	return ServerStatsPeriodStore.Transaction(func(st *models.StatsPeriodStore) error {
		for _, model := range channelAuthorStats {
			err := st.Insert(model)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func UpdateOldStats() {
	started := time.Now()
	del, err := ServerStatsPeriodStore.RawExec("DELETE FROM server_stats_periods WHERE started < NOW() - INTERVAL '2 days'")
	log.Infof("ServerStats: Deleted %d records in %s", del, time.Since(started))
	if err != nil {
		log.WithError(err).Error("ServerStats: Failed deleting old stats")
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

func RetrieveFullStats(client *redis.Client, guildID int64) (*FullStats, error) {
	// Query the short term stats and the long term stats
	// TODO: If we start moving them over in between we will get somehwat incorrect stats
	// not sure how to fix other than locking

	stats, err := RetrieveRedisStats(client, guildID)
	if err != nil {
		return nil, err
	}

	rows, err := ServerStatsPeriodStore.FindAll(models.NewStatsPeriodQuery().FindByGuildID(
		kallax.Eq, guildID).FindByStarted(kallax.Gt, time.Now().Add(time.Hour*-24)))

	if err != nil && err != kallax.ErrNotFound {
		return nil, err
	}

	// Merge the stats togheter
	for _, period := range rows {
		stringedChannel := strconv.FormatInt(period.ChannelID, 10)
		if st, ok := stats.ChannelsHour[stringedChannel]; ok {
			st.Count += period.Count
		} else {
			stats.ChannelsHour[stringedChannel] = &ChannelStats{
				Name:  stringedChannel,
				Count: period.Count,
			}
		}
	}

	return stats, nil
}

func RetrieveRedisStats(client *redis.Client, guildID int64) (*FullStats, error) {
	now := time.Now()
	yesterday := now.Add(time.Hour * -24)
	unixYesterday := yesterday.Unix()

	strGID := discordgo.StrID(guildID)
	client.PipeAppend("ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+strGID, unixYesterday, "+inf")
	client.PipeAppend("ZCOUNT", "guild_stats_members_joined_day:"+strGID, unixYesterday, "+inf")
	client.PipeAppend("ZCOUNT", "guild_stats_members_left_day:"+strGID, unixYesterday, "+inf")
	client.PipeAppend("SCARD", "guild_stats_online:"+strGID)

	replies, err := common.GetRedisReplies(client, 4)
	if err != nil {
		return nil, err
	}

	messageStatsRaw, err := replies[0].List()
	if err != nil {
		return nil, err
	}

	channelResult, err := GetChannelMessageStats(client, messageStatsRaw, guildID)
	if err != nil {
		return nil, err
	}

	joined, err := replies[1].Int()
	if err != nil {
		return nil, err
	}

	left, err := replies[2].Int()
	if err != nil {
		return nil, err
	}

	online, err := replies[3].Int()
	if err != nil {
		return nil, err
	}

	members := 0
	reply := client.Cmd("GET", "guild_stats_num_members:"+strGID)
	if !reply.IsType(redis.Nil) {
		var err error
		members, err = reply.Int()
		if err != nil {
			return nil, err
		}

	}

	stats := &FullStats{
		ChannelsHour: channelResult,
		JoinedDay:    joined,
		LeftDay:      left,
		Online:       online,
		TotalMembers: members,
	}

	return stats, nil
}

func GetChannelMessageStats(client *redis.Client, raw []string, guildID int64) (map[string]*ChannelStats, error) {

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

var _ configstore.PostFetchHandler = (*ServerStatsConfig)(nil)

type ServerStatsConfig struct {
	configstore.GuildConfigModel
	Public         bool
	IgnoreChannels string

	ParsedChannels []int64 `gorm:"-"`
}

func (c *ServerStatsConfig) GetName() string {
	return "server_stats_config"
}

func (s *ServerStatsConfig) PostFetch() {
	split := strings.Split(s.IgnoreChannels, ",")
	for _, v := range split {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			s.ParsedChannels = append(s.ParsedChannels, parsed)
		}
	}
}

func GetConfig(ctx context.Context, GuildID int64) (*ServerStatsConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var conf ServerStatsConfig
	err := configstore.Cached.GetGuildConfig(ctx, GuildID, &conf)
	if err != nil && err != configstore.ErrNotFound {
		return &conf, err
	}

	return &conf, nil
}
