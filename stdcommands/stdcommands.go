package stdcommands

import (
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/stdcommands/define"
	"github.com/jonas747/yagpdb/stdcommands/reverse"
	"github.com/jonas747/yagpdb/stdcommands/weather"
	"github.com/jonas747/yagpdb/stdcommands/calc"
	"github.com/jonas747/yagpdb/stdcommands/topic"
	"github.com/jonas747/yagpdb/stdcommands/catfact"
	"github.com/jonas747/yagpdb/stdcommands/advice"
	"github.com/jonas747/yagpdb/stdcommands/ping"
	"github.com/jonas747/yagpdb/stdcommands/throw"
	"github.com/jonas747/yagpdb/stdcommands/roll"
	"github.com/jonas747/yagpdb/stdcommands/customembed"
	"github.com/jonas747/yagpdb/stdcommands/currenttime"
	"github.com/jonas747/yagpdb/stdcommands/mentionrole"
	"github.com/jonas747/yagpdb/stdcommands/listroles"
	"github.com/jonas747/yagpdb/stdcommands/wouldyourather"
	"github.com/jonas747/yagpdb/stdcommands/stateinfo"
	"github.com/jonas747/yagpdb/stdcommands/secretcommand"
	"github.com/jonas747/yagpdb/stdcommands/leaveserver"
	"github.com/jonas747/yagpdb/stdcommands/banserver"
	"github.com/jonas747/yagpdb/stdcommands/unbanserver"
	"github.com/jonas747/yagpdb/stdcommands/topservers"
	"github.com/jonas747/yagpdb/stdcommands/topcommands"
	"github.com/jonas747/yagpdb/stdcommands/topevents"
	"github.com/jonas747/yagpdb/stdcommands/currentshard"
	"github.com/jonas747/yagpdb/stdcommands/memberfetcher"
	"github.com/jonas747/yagpdb/stdcommands/info"
	"github.com/jonas747/yagpdb/stdcommands/invite"
	"github.com/jonas747/yagpdb/stdcommands/yagstatus"
)

type Command interface {
  EventHandler() ([]eventsystem.Event, eventsystem.Handler)
  YAGCommand() *commands.YAGCommand
}

type Plugin struct{}

func (p *Plugin) InitBot() {
  cmds := []Command {
    // Info
    info.Cmd(),
    invite.Cmd(),

    // Standard
    define.Cmd(),
    reverse.Cmd(),
    weather.Cmd(),
    calc.Cmd(),
    topic.Cmd(),
    catfact.Cmd(),
    advice.Cmd(),
    ping.Cmd(),
    throw.Cmd(),
    roll.Cmd(),
    customembed.Cmd(),
    currenttime.Cmd(),
    mentionrole.Cmd(),
    listroles.Cmd(),
    wouldyourather.Cmd(),

    // Maintenance
    stateinfo.Cmd(),
    secretcommand.Cmd(),
    leaveserver.Cmd(),
    banserver.Cmd(),
    unbanserver.Cmd(),
    topservers.Cmd(),
    topcommands.Cmd(),
    topevents.Cmd(),
    currentshard.Cmd(),
    memberfetcher.Cmd(),
    yagstatus.Cmd(),
  }

  for _, cmd := range cmds {
    evts, handler := cmd.EventHandler()
	  eventsystem.AddHandler(handler, evts...)
    commands.AddRootCommands(cmd.YAGCommand())
  }
}

func (p *Plugin) Name() string {
	return "stdcommands"
}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}
