package stdcommands

import (
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/advice"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/allocstat"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/banserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/calc"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/catfact"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/ccreqs"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/cleardm"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/createinvite"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/currentshard"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/currenttime"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/customembed"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dadjoke"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dcallvoice"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/define"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dictionary"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/dogfact"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/eightball"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/findserver"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/forex"
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
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/ping"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/poll"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/roast"
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
		calc.Command,
		ping.Command,
		customembed.Command,
		simpleembed.Command,
		currenttime.Command,
		listroles.Command,
		memstats.Command,
		poll.Command,
		undelete.Command,
		viewperms.Command,
		topgames.Command,

		// Maintenance
		stateinfo.Command,
		leaveserver.Command,
		banserver.Command,
		cleardm.Command,
		allocstat.Command,
		unbanserver.Command,
		topservers.Command,
		topcommands.Command,
		topevents.Command,
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
	guildCommands(p)
	funCommands(p)
}

var funCommandList = []*commands.YAGCommand{
	eightball.Command, advice.Command, catfact.Command, dadjoke.Command,
	dogfact.Command, define.Command, dictionary.Command, forex.Command,
	howlongtobeat.Command, inspire.Command, roast.Command, roll.Command,
	throw.Command, topic.Command, weather.Command, wouldyourather.Command,
	xkcd.Command,
}

func funCommands(p *Plugin) {
	container, _ := commands.CommandSystem.Root.Sub("fun")
	container.Description = "Fun and lookup commands"

	for _, cmd := range funCommandList {
		// the container owns the slash surface for these now
		cmd.SlashCommandEnabled = false

		legacy := append([]string{cmd.Name}, cmd.Aliases...)
		commands.AddContainerCommand(container, cmd)
		commands.AddRootAliases(p, cmd, legacy...)
	}

	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}

func guildCommands(p *Plugin) {
	container, _ := commands.CommandSystem.Root.Sub("guild")
	container.Description = "Guild utilities"

	for _, cmd := range []*commands.YAGCommand{currentshard.Command, guildunavailable.Command} {
		cmd.Plugin = p
		commands.AddContainerCommand(container, cmd)
	}

	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})

	commands.AddRootAliases(p, currentshard.Command, "cshard", "currentshard")
	commands.AddRootAliases(p, guildunavailable.Command, "isguildunavailable")
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
