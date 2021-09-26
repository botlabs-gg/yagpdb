package globalrl

import (
	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/stdcommands/util"
	"github.com/jonas747/dcmd/v4"
	"github.com/jonas747/discordgo/v2"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	Name:                 "globalrl",
	Description:          "Tests the global ratelimit functionality",
	RequiredArgs:         1,
	HideFromHelp:         true,
	HideFromCommandsPage: true,
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {

		rlEvt := &discordgo.RateLimit{
			URL: "Wew",
			TooManyRequests: &discordgo.TooManyRequests{
				Bucket:     "wewsss",
				Message:    "Too many!",
				RetryAfter: 5,
			},
		}

		go common.BotSession.HandleEvent("__RATE_LIMIT__", rlEvt)

		return "Done", nil
	}),
}
