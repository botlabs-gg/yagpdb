package bot

import (
	"sync"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardorchestrator"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
)

const (
	// How long after removing a guild the config for it gets cleared
	GuildRemoveConfigExpire = 60 * 60 * 24 // <- 1 day
)

// Used for deleting configuration about servers
type RemoveGuildHandler interface {
	RemoveGuild(guildID int64) error
}

// Used for intializing stuff for new servers
type NewGuildHandler interface {
	NewGuild(guild *discordgo.Guild) error
}

// Fired when the bot it starting up, not for the webserver
type BotInitHandler interface {
	BotInit()
}

// Fired when the bot it starting up, after BotInit
type LateBotInitHandler interface {
	LateBotInit()
}

// BotStopperHandler runs when the bot is shuttdown down
// you need to call wg.Done when you have completed your plugin shutdown (stopped background workers)
type BotStopperHandler interface {
	StopBot(wg *sync.WaitGroup)
}

type ShardMigrationHandler interface {
	GuildMigrated(guild *dstate.GuildState, toThisSlave bool)
}

func EmitGuildRemoved(guildID int64) {
	for _, v := range common.Plugins {
		if remover, ok := v.(RemoveGuildHandler); ok {
			err := remover.RemoveGuild(guildID)
			if err != nil {
				logger.WithError(err).Error("Error Running RemoveGuild on ", v.PluginInfo().Name)
			}
		}
	}
}

type ShardMigrationSender interface {
	ShardMigrationSend(shard int) int
}

type ShardMigrationReceiver interface {
	ShardMigrationReceive(evt dshardorchestrator.EventType, data interface{})
}

// bot plugin
var logger = common.GetPluginLogger(&botPlugin{})

type botPlugin struct {
}

func (p *botPlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Bot Core",
		SysName:  "bot_core",
		Category: common.PluginCategoryCore,
	}
}
