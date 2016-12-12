package reddit

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
)

func (p *Plugin) InitBot() {}

func (p *Plugin) RemoveGuild(c *redis.Client, g string) error {
	config, err := GetConfig(c, "guild_subreddit_watch:"+g)
	if err != nil {
		return err
	}
	for _, v := range config {
		v.Remove(c)
	}
	logrus.Info("Removed reddit config for deleted guild")
	return nil
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
