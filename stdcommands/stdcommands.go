package stdcommands

import (
	"github.com/botlabs-gg/yagpdb/bot"
	"github.com/botlabs-gg/yagpdb/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/stdcommands/advice"
	"github.com/botlabs-gg/yagpdb/stdcommands/allocstat"
	"github.com/botlabs-gg/yagpdb/stdcommands/banserver"
	"github.com/botlabs-gg/yagpdb/stdcommands/calc"
	"github.com/botlabs-gg/yagpdb/stdcommands/catfact"
	"github.com/botlabs-gg/yagpdb/stdcommands/ccreqs"
	"github.com/botlabs-gg/yagpdb/stdcommands/createinvite"
	"github.com/botlabs-gg/yagpdb/stdcommands/currentshard"
	"github.com/botlabs-gg/yagpdb/stdcommands/currenttime"
	"github.com/botlabs-gg/yagpdb/stdcommands/customembed"
	"github.com/botlabs-gg/yagpdb/stdcommands/dadjoke"
	"github.com/botlabs-gg/yagpdb/stdcommands/dcallvoice"
	"github.com/botlabs-gg/yagpdb/stdcommands/define"
	"github.com/botlabs-gg/yagpdb/stdcommands/dogfact"
	"github.com/botlabs-gg/yagpdb/stdcommands/findserver"
	"github.com/botlabs-gg/yagpdb/stdcommands/globalrl"
	"github.com/botlabs-gg/yagpdb/stdcommands/guildunavailable"
	"github.com/botlabs-gg/yagpdb/stdcommands/howlongtobeat"
	"github.com/botlabs-gg/yagpdb/stdcommands/info"
	"github.com/botlabs-gg/yagpdb/stdcommands/invite"
	"github.com/botlabs-gg/yagpdb/stdcommands/leaveserver"
	"github.com/botlabs-gg/yagpdb/stdcommands/listflags"
	"github.com/botlabs-gg/yagpdb/stdcommands/listroles"
	"github.com/botlabs-gg/yagpdb/stdcommands/memstats"
	"github.com/botlabs-gg/yagpdb/stdcommands/ping"
	"github.com/botlabs-gg/yagpdb/stdcommands/poll"
	"github.com/botlabs-gg/yagpdb/stdcommands/roll"
	"github.com/botlabs-gg/yagpdb/stdcommands/setstatus"
	"github.com/botlabs-gg/yagpdb/stdcommands/simpleembed"
	"github.com/botlabs-gg/yagpdb/stdcommands/sleep"
	"github.com/botlabs-gg/yagpdb/stdcommands/statedbg"
	"github.com/botlabs-gg/yagpdb/stdcommands/stateinfo"
	"github.com/botlabs-gg/yagpdb/stdcommands/throw"
	"github.com/botlabs-gg/yagpdb/stdcommands/toggledbg"
	"github.com/botlabs-gg/yagpdb/stdcommands/topcommands"
	"github.com/botlabs-gg/yagpdb/stdcommands/topevents"
	"github.com/botlabs-gg/yagpdb/stdcommands/topgames"
	"github.com/botlabs-gg/yagpdb/stdcommands/topic"
	"github.com/botlabs-gg/yagpdb/stdcommands/topservers"
	"github.com/botlabs-gg/yagpdb/stdcommands/unbanserver"
	"github.com/botlabs-gg/yagpdb/stdcommands/undelete"
	"github.com/botlabs-gg/yagpdb/stdcommands/viewperms"
	"github.com/botlabs-gg/yagpdb/stdcommands/weather"
	"github.com/botlabs-gg/yagpdb/stdcommands/wouldyourather"
	"github.com/botlabs-gg/yagpdb/stdcommands/xkcd"
	"github.com/botlabs-gg/yagpdb/stdcommands/yagstatus"
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
		invite.Command,

		// Standard
		define.Command,
		weather.Command,
		calc.Command,
		topic.Command,
		catfact.Command,
		dadjoke.Command,
		dogfact.Command,
		advice.Command,
		ping.Command,
		throw.Command,
		roll.Command,
		customembed.Command,
		simpleembed.Command,
		currenttime.Command,
		listroles.Command,
		memstats.Command,
		wouldyourather.Command,
		poll.Command,
		undelete.Command,
		viewperms.Command,
		topgames.Command,
		xkcd.Command,
		howlongtobeat.Command,

		// Maintenance
		stateinfo.Command,
		leaveserver.Command,
		banserver.Command,
		allocstat.Command,
		unbanserver.Command,
		topservers.Command,
		topcommands.Command,
		topevents.Command,
		currentshard.Command,
		guildunavailable.Command,
		yagstatus.Command,
		setstatus.Command,
		createinvite.Command,
		findserver.Command,
		dcallvoice.Command,
		ccreqs.Command,
		sleep.Command,
		toggledbg.Command,
		globalrl.Command,
		listflags.Command,
	)

	statedbg.Commands()

}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, ping.HandleMessageCreate, eventsystem.EventMessageCreate)
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
