package premium

import (
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func init() {
	oldF := bot.StateLimitsF
	bot.StateLimitsF = func(guildID int64) (int, time.Duration) {
		premium, err := IsGuildPremiumCached(guildID)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Failed checking if guild is premium")
			return oldF(guildID)
		}

		if premium {
			return PremiumStateMaxMessags, PremiumStateMaxMessageAge
		}

		return oldF(guildID)
	}
}

func (p *Plugin) BotInit() {
	// bot.State.CustomLimitProvider = p
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmdGenerateCode)
}

const (
	PremiumStateMaxMessags    = 10000
	PremiumStateMaxMessageAge = time.Hour * 12
)
