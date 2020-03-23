package premium

import (
	"time"

	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	bot.State.CustomLimitProvider = p
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdGenerateCode)
}

const (
	NormalStateMaxMessages   = 1000
	NormalStateMaxMessageAge = time.Hour

	PremiumStateMaxMessags    = 10000
	PremiumStateMaxMessageAge = time.Hour * 12
)

func (p *Plugin) MessageLimits(cs *dstate.ChannelState) (maxMessages int, maxMessageAge time.Duration) {
	if cs.Guild == nil {
		return NormalStateMaxMessages, NormalStateMaxMessageAge
	}

	premium, err := IsGuildPremiumCached(cs.Guild.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", cs.Guild.ID).Error("Failed checking if guild is premium")
	}

	if premium {
		return PremiumStateMaxMessags, PremiumStateMaxMessageAge
	}

	return NormalStateMaxMessages, NormalStateMaxMessageAge
}
