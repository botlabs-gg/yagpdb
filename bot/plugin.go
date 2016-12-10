package bot

import (
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
	RemoveGuild(client *redis.Client, guild *discordgo.Guild) error
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

	common.AddPlugin(plugin)
}

func EmitGuildRemoved(client *redis.Client, guild *discordgo.Guild) {
	for _, v := range Plugins {
		if remover, ok := v.(RemoveGuildHandler); ok {
			remover.RemoveGuild(client, guild)
		}
	}
}
