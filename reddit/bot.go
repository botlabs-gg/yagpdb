package reddit

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

func (p *Plugin) InitBot() {
	common.BotSession.AddHandler(bot.CustomGuildDelete(OnGuildRemove))
}

func OnGuildRemove(s *discordgo.Session, g *discordgo.GuildDelete, c *redis.Client) {
	config, err := GetConfig(c, "guild_subreddit_watch:"+g.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving reddit config")
	}
	for _, v := range config {
		v.Remove(c)
	}
}

func (p *Plugin) Status(client *redis.Client) (string, string) {
	subs := 0
	channels := 0
	cursor := "0"
	for {
		reply := client.Cmd("SCAN", cursor, "MATCH", "global_subreddit_watch:*")
		if reply.Err != nil {
			logrus.WithError(reply.Err).Error("Error scanning")
			break
		}

		if len(reply.Elems) < 2 {
			logrus.Error("Invalid scan")
			break
		}

		newCursor, err := reply.Elems[0].Str()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving new cursor")
			break
		}
		cursor = newCursor

		list, err := reply.Elems[1].List()
		if err != nil {
			logrus.WithError(err).Error("Failed retrieving list")
			break
		}

		for _, key := range list {
			config, err := GetConfig(client, key)
			if err != nil {
				logrus.WithError(err).Error("Failed reading global config")
				continue
			}
			if len(config) < 1 {
				continue
			}
			subs++
			channels += len(config)
		}

		if cursor == "" || cursor == "0" {
			break
		}
	}

	return "Subs/Channels", fmt.Sprintf("%d/%d", subs, channels)
}
