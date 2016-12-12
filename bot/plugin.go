package bot

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
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

var Plugins []Plugin

// Register a plugin, should only be called before webserver is started!!!
func RegisterPlugin(plugin Plugin) {
	if Plugins == nil {
		Plugins = []Plugin{plugin}
	} else {
		Plugins = append(Plugins, plugin)
	}

	if _, ok := plugin.(BotStarterHandler); ok {
		logrus.Info(plugin.Name(), " is a BotStarter")
	}
	if _, ok := plugin.(BotStopperHandler); ok {
		logrus.Info(plugin.Name(), " is a BotStopper")
	}
	if _, ok := plugin.(RemoveGuildHandler); ok {
		logrus.Info(plugin.Name(), " is a RemoveGuildHandler")
	}

	common.AddPlugin(plugin)
}

func EmitGuildRemoved(client *redis.Client, guildID string) {
	for _, v := range Plugins {
		if remover, ok := v.(RemoveGuildHandler); ok {
			err := remover.RemoveGuild(client, guildID)
			if err != nil {
				logrus.WithError(err).Error("Error Running RemoveGuild on ", v.Name())
			}
		}
	}
}
