package stdcommands

import (
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

type Plugin struct{}

func (p *Plugin) InitBot() {
	eventsystem.AddHandler(HandleMessageCreate, eventsystem.EventMessageCreate)
	commands.CommandSystem.RegisterCommands(
		cmdInfo,
		cmdInvite,
	)

	commands.CommandSystem.RegisterCommands(generalCommands...)
	commands.CommandSystem.RegisterCommands(maintenanceCommands...)
}

func (p *Plugin) Name() string {
	return "stdcommands"
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
