package stdcommands

import (
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/calc"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/cleardm"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/currenttime"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/define"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/info"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/listroles"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/ping"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/roll"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/setstatus"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topgames"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topic"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/undelete"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/viewperms"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/yagstatus"
)

var (
	_ bot.BotInitHandler       = (*Plugin)(nil)
	_ commands.CommandProvider = (*Plugin)(nil)
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Standard Commands",
		SysName:  "standard_commands",
		Category: common.PluginCategoryCore,
	}
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p,
		// Info
		info.Command,

		// Standard
		define.Command,
		calc.Command,
		topic.Command,
		ping.Command,
		roll.Command,
		currenttime.Command,
		listroles.Command,
		undelete.Command,
		viewperms.Command,
		topgames.Command,

		// Maintenance
		cleardm.Command,
		yagstatus.Command,
		setstatus.Command,
	)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, ping.HandleMessageCreate, eventsystem.EventMessageCreate)
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
