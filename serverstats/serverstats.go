package serverstats

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"strings"
	"time"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Server Stats"
}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)

	common.SQL.AutoMigrate(&ServerStatsConfig{})
	configstore.RegisterConfig(configstore.SQL, &ServerStatsConfig{})
}

func (p *Plugin) StartBot() {
	go UpdateStatsLoop()
}

// Removes expired stats on a interval
func UpdateStatsLoop() {
	client, _ := common.RedisPool.Get()
	for {
		if client == nil {
			var err error
			client, err = common.RedisPool.Get()
			if err != nil {
				log.WithError(err).Error("Failed retrieving redis connection")
				time.Sleep(time.Second)
				continue
			}
		}

		started := time.Now()
		client, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).Error("Failed retrieving redis connection")
			time.Sleep(time.Second)
			client = nil
			continue
		}

		guilds, err := client.Cmd("SMEMBERS", "connected_guilds").List()
		if err != nil {
			log.WithError(err).Error("Failed retrieving connected guilds")
			time.Sleep(time.Second)
			client = nil
			continue
		}

		for _, g := range guilds {
			err = UpdateStats(client, g)
			if err != nil {
				log.WithFields(log.Fields{
					"guild":      g,
					log.ErrorKey: err,
				}).Error("Failed updating stats")
			}
		}

		log.WithFields(log.Fields{
			"duration":    time.Since(started).Seconds(),
			"num_servers": len(guilds),
		}).Info("Updated stats")

		time.Sleep(time.Minute)
	}
}

// Updates the stats on a specific guild, removing expired stats
func UpdateStats(client *redis.Client, guildID string) error {
	now := time.Now()
	yesterday := now.Add(time.Hour * -24)
	unixYesterday := yesterday.Unix()

	client.Append("ZREMRANGEBYSCORE", "guild_stats_msg_channel_day:"+guildID, "-inf", unixYesterday)
	client.Append("ZREMRANGEBYSCORE", "guild_stats_members_joined_day:"+guildID, "-inf", unixYesterday)
	client.Append("ZREMRANGEBYSCORE", "guild_stats_members_left_day:"+guildID, "-inf", unixYesterday)

	_, err := common.GetRedisReplies(client, 3)
	return err
}

type ChannelStats struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type FullStats struct {
	ChannelsHour map[string]*ChannelStats `json:"channels_hour"`
	JoinedDay    int                      `json:"joined_day"`
	LeftDay      int                      `json:"left_day"`
	Online       int                      `json:"online_now"`
	TotalMembers int                      `json:"total_members_now"`
}

func RetrieveFullStats(client *redis.Client, guildID string) (*FullStats, error) {
	now := time.Now()
	yesterday := now.Add(time.Hour * -24)
	unixYesterday := yesterday.Unix()

	client.Append("ZRANGEBYSCORE", "guild_stats_msg_channel_day:"+guildID, unixYesterday, "+inf")
	client.Append("ZCOUNT", "guild_stats_members_joined_day:"+guildID, unixYesterday, "+inf")
	client.Append("ZCOUNT", "guild_stats_members_left_day:"+guildID, unixYesterday, "+inf")
	client.Append("SCARD", "guild_stats_online:"+guildID)

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
	reply := client.Cmd("GET", "guild_stats_num_members:"+guildID)
	if reply.Type != redis.NilReply {
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

func GetChannelMessageStats(client *redis.Client, raw []string, guildID string) (map[string]*ChannelStats, error) {
	channels, err := common.GetGuildChannels(client, guildID)
	if err != nil {
		return nil, err
	}

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
			// Make it human readable
			for _, c := range channels {
				if c.ID == channelID {
					name = c.Name
					break
				}
			}

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

	ParsedChannels []string `gorm:"-"`
}

func (c *ServerStatsConfig) GetName() string {
	return "server_stats_config"
}

func (s *ServerStatsConfig) PostFetch() {
	s.ParsedChannels = strings.Split(s.IgnoreChannels, ",")
}

func GetConfig(ctx context.Context, GuildID string) (*ServerStatsConfig, error) {
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
