package serverstats

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"log"
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
}

// Removes expired stats on a interval
func UpdateStatsLoop() {
	for {
		started := time.Now()
		client, err := common.RedisPool.Get()
		if err != nil {
			log.Println("Failed retreiving redis conn")
			time.Sleep(time.Second)
			continue
		}

		guilds, err := client.Cmd("SMEMBERS", "connected_guilds").List()
		if err != nil {
			log.Println("Failed retrieving connected guilds", err)
			time.Sleep(time.Second)
			continue
		}

		for _, g := range guilds {
			err = UpdateStats(client, g)
			if err != nil {
				log.Println("Failed updating stats for ", g, err)
			}
		}
		common.RedisPool.CarefullyPut(client, &err)
		log.Println("Took", time.Since(started), "To update stats for", len(guilds), "servers")
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

	members, err := client.Cmd("GET", "guild_stats_num_members:"+guildID).Int()
	if err != nil {
		if _, ok := err.(*redis.CmdError); !ok {
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
			log.Println("Invalid message stats", guildID, result)
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
