package bot

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/sirupsen/logrus"
	"sync"
)

const (
	// How long after removing a guild the config for it gets cleared
	GuildRemoveConfigExpire = 60 * 60 * 24 // <- 1 day
)

// Used for deleting configuration about servers
type RemoveGuildHandler interface {
	RemoveGuild(client *redis.Client, guildID int64) error
}

// Used for intializing stuff for new servers
type NewGuildHandler interface {
	NewGuild(client *redis.Client, guild *discordgo.Guild) error
}

// Fired when the bot it starting up, not for the webserver
type BotInitHandler interface {
	BotInit()
}

// Fired after the bot has connected all shards
type BotStartedHandler interface {
	BotStarted()
}

type BotStopperHandler interface {
	StopBot(wg *sync.WaitGroup)
}

type ShardMigrationHandler interface {
	GuildMigrated(guild *dstate.GuildState, toThisSlave bool)
}

func EmitGuildRemoved(guildID int64) {
	client := common.MustGetRedisClient()
	defer common.RedisPool.Put(client)

	for _, v := range common.Plugins {
		if remover, ok := v.(RemoveGuildHandler); ok {
			err := remover.RemoveGuild(client, guildID)
			if err != nil {
				logrus.WithError(err).Error("Error Running RemoveGuild on ", v.Name())
			}
		}
	}
}
