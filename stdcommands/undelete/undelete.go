package undelete

import (
	"fmt"
	"time"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Undelete",
	Aliases:      []string{"ud"},
	Description:  "Views your recent deleted messages, or all users deleted messages (with \"-a\" and manage messages perm) in this channel",
	RequiredArgs: 0,
	ArgSwitches: []*dcmd.ArgDef{
		{Switch: "a", Name: "all"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		allUsers := data.Switch("a").Value != nil && data.Switch("a").Value.(bool)

		if allUsers {
			if ok, err := bot.AdminOrPermMS(data.CS.ID, data.MS, discordgo.PermissionManageMessages); !ok || err != nil {
				if err != nil {
					return nil, err
				} else if !ok {
					return "You need `Manage Messages` permissions to view all users deleted messages", nil
				}
			}
		}

		resp := "Up to 10 last deleted messages (last hour or 12 hours for premium): \n\n"
		numFound := 0

		data.GS.RLock()
		defer data.GS.RUnlock()

		for i := len(data.CS.Messages) - 1; i >= 0 && numFound < 10; i-- {
			msg := data.CS.Messages[i]

			if !msg.Deleted {
				continue
			}

			if !allUsers && msg.Author.ID != data.Msg.Author.ID {
				continue
			}

			precision := common.DurationPrecisionHours
			if time.Since(msg.ParsedCreated) < time.Hour {
				precision = common.DurationPrecisionMinutes
				if time.Since(msg.ParsedCreated) < time.Minute {
					precision = common.DurationPrecisionSeconds
				}
			}

			// Match found!
			timeSince := common.HumanizeDuration(precision, time.Since(msg.ParsedCreated))

			resp += fmt.Sprintf("`%s ago (%s)` **%s**#%s: %s\n\n", timeSince, msg.ParsedCreated.UTC().Format(time.ANSIC), msg.Author.Username, msg.Author.Discriminator, msg.ContentWithMentionsReplaced())
			numFound++
		}

		if numFound == 0 {
			resp += "none..."
		}

		return resp, nil
	},
}
