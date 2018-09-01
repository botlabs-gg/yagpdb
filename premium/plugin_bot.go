package premium

import (
	"github.com/jonas747/yagpdb/bot"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	go run()
}
