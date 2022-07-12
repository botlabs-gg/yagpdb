package stdcommands

import (
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/advice"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/allocstat"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/banserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/calc"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/catfact"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/ccreqs"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/createinvite"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/currentshard"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/currenttime"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/customembed"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dadjoke"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dcallvoice"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/define"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dogfact"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/findserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/globalrl"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/guildunavailable"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/howlongtobeat"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/info"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/inspire"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/invite"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/leaveserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/listflags"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/listroles"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/memstats"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/owldictionary"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/ping"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/poll"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/roll"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/setstatus"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/simpleembed"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/sleep"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/statedbg"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/stateinfo"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/throw"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/toggledbg"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topcommands"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topevents"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topgames"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topic"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/topservers"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/unbanserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/undelete"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/viewperms"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/weather"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/wouldyourather"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/xkcd"
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
		inspire.Command,

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

	if !owldictionary.ShouldRegister() {
		common.GetPluginLogger(p).Warn("Owlbot API token not provided, skipping adding dictionary command...")
		return
	}

	commands.AddRootCommands(p, owldictionary.Command)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, ping.HandleMessageCreate, eventsystem.EventMessageCreate)
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
