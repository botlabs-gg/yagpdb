package stdcommands

import (
	"github.com/RhykerWells/yagpdb/v2/bot"
	"github.com/RhykerWells/yagpdb/v2/bot/eventsystem"
	"github.com/RhykerWells/yagpdb/v2/commands"
	"github.com/RhykerWells/yagpdb/v2/common"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/calc"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/cleardm"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/currenttime"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/define"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/info"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/listroles"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/ping"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/roll"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/setstatus"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/topgames"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/topic"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/undelete"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/viewperms"
	"github.com/RhykerWells/yagpdb/v2/stdcommands/yagstatus"
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
