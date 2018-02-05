package bot

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"sync"
)

const (
	// How long after removing a guild the config for it gets cleared
	GuildRemoveConfigExpire = 60 * 60 * 24 // <- 1 day
)

type Plugin interface {
	// Called when the plugin is supposed to be initialized
	// That is add comnands, discord event handlers
	InitBot()
	Name() string
}

// Used for deleting configuration about servers
type RemoveGuildHandler interface {
	RemoveGuild(client *redis.Client, guildID string) error
}

// Used for intializing stuff for new servers
type NewGuildHandler interface {
	NewGuild(client *redis.Client, guild *discordgo.Guild) error
}

type BotStarterHandler interface {
	StartBot()
}
type BotStopperHandler interface {
	StopBot(wg *sync.WaitGroup)
}

func EmitGuildRemoved(client *redis.Client, guildID string) {
	for _, v := range common.Plugins {
		if remover, ok := v.(RemoveGuildHandler); ok {
			err := remover.RemoveGuild(client, guildID)
			if err != nil {
				logrus.WithError(err).Error("Error Running RemoveGuild on ", v.Name())
			}
		}
	}
}
