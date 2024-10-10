package undelete

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryTool,
	Name:            "Undelete",
	Aliases:         []string{"ud", "snipe"},
	Description:     "Views the recent deleted messages. By default, only the current user's deleted messages will show.",
	LongDescription: "You can use the `-a` flag to view all users delete messages, or `-u` to view a specified user's deleted messages.\nBoth `-a` and `-u` require Manage Messages permission.\nNote: `-u` overrides `-a` meaning even though `-a` might've been specified along with `-u` only messages from the user provided using `-u` will be shown.",
	RequiredArgs:    0,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "a", Help: "from all users"},
		{Name: "u", Help: "from a specific user", Type: dcmd.UserID, Default: 0},
		{Name: "count", Help: "Number of messages to show, Min: 1, Max 10", Type: dcmd.Int, Default: 10},
		{Name: "channel", Help: "Optional target channel", Type: dcmd.Channel},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		allUsers := data.Switch("a").Value != nil && data.Switch("a").Value.(bool)
		targetUser := data.Switch("u").Int64()

		channel := data.GuildData.CS

		if data.Switch("channel").Value != nil {
			channel = data.Switch("channel").Value.(*dstate.ChannelState)

			ok, err := bot.AdminOrPermMS(data.GuildData.GS.ID, channel.ID, data.GuildData.MS, discordgo.PermissionViewChannel)
			if err != nil {
				return nil, err
			} else if !ok {
				return "You do not have permission to view that channel.", nil
			}
		}

		if allUsers || targetUser != 0 {
			ok, err := bot.AdminOrPermMS(data.GuildData.GS.ID, channel.ID, data.GuildData.MS, discordgo.PermissionManageMessages)
			if err != nil {
				return nil, err
			} else if !ok && targetUser == 0 {
				return "You need `Manage Messages` permissions to view all users deleted messages.", nil
			} else if !ok {
				return "You need `Manage Messages` permissions to target a specific user other than yourself.", nil
			}
		}
		count := data.Switch("count").Int()
		if count > 10 {
			count = 10
		} else if count < 1 {
			count = 1
		}

		resp := fmt.Sprintf("Last %d deleted message(s) (last hour or 12 hours for premium): \n\n", count)
		numFound := 0

		messages := bot.State.GetMessages(data.GuildData.GS.ID, channel.ID, &dstate.MessagesQuery{Limit: 100, IncludeDeleted: true})
		for _, msg := range messages {
			if !msg.Deleted {
				continue
			}

			if !allUsers && msg.Author.ID != data.Author.ID && targetUser == 0 {
				continue
			}

			if targetUser != 0 && msg.Author.ID != targetUser {
				continue
			}

			parsedTime := fmt.Sprintf("<t:%s:f>", fmt.Sprint(msg.ParsedCreatedAt.UTC().Unix()))
			relativeTime := fmt.Sprintf("<t:%s:R>", fmt.Sprint(msg.ParsedCreatedAt.UTC().Unix()))

			resp += fmt.Sprintf("%s (%s) **%s** (ID %d): %s\n\n", parsedTime, relativeTime, msg.Author.String(), msg.Author.ID, msg.ContentWithMentionsReplaced())
			numFound++
			if numFound == count {
				break
			}
		}

		if numFound == 0 {
			resp += "none..."
		}

		return resp, nil
	},
}
