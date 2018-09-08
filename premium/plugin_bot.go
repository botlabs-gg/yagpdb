package premium

import (
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	go runMonitor()
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(cmdGenerateCode)
}
