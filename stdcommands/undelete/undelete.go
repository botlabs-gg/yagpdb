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
	Description:  "Views the first 10 recent deleted messages. By default only the current users deleted messages will show.\n\nUse `-a` to view all users deleted messages or `-u` to view a specific users deleted messages.\nBoth `-a` and `-u` require \"Manage Messages\" permission.",
	RequiredArgs: 0,
	ArgSwitches: []*dcmd.ArgDef{
		{Switch: "a", Name: "all"},
		&dcmd.ArgDef{Switch: "u", Name: "user", Type: dcmd.UserID, Default: 0},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		allUsers := data.Switch("a").Value != nil && data.Switch("a").Value.(bool)
		targetUser := data.Switch("u").Int64()
		
		if allUsers {
			if ok, err := bot.AdminOrPermMS(data.CS.ID, data.MS, discordgo.PermissionManageMessages); !ok || err != nil {
				if err != nil {
					return nil, err
				} else if !ok {
					return "You need `Manage Messages` permissions to view all users deleted messages", nil
				}
			}
		}
		
		if targetUser != 0 {
			if ok, err := bot.AdminOrPermMS(data.CS.ID, data.MS, discordgo.PermissionManageMessages); err != nil || !ok && data.MS.ID != targetUser {
				if err != nil {
					return nil, err
				} else if !ok && data.MS.ID != targetUser {
					return "You need `Manage Messages` permissions to target a specific user other than yourself.", nil
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
			
			if !allUsers && msg.Author.ID != data.Msg.Author.ID && targetUser == 0 {
				continue
			}
			
			if targetUser != 0 && msg.Author.ID != targetUser {
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
