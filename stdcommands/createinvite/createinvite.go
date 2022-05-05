package createinvite

import (
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "createinvite",
	Description:          "Maintenance command, creates a invite for the specified server",
	HideFromHelp:         true,
	RequiredArgs:         1,
	Arguments: []*dcmd.ArgDef{
		{Name: "server", Type: dcmd.BigInt},
	},
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		channels, err := common.BotSession.GuildChannels(data.Args[0].Int64())
		if err != nil {
			return nil, err
		}

		channelID := int64(0)
		for _, v := range channels {
			if v.Type == discordgo.ChannelTypeGuildText {
				channelID = v.ID
				break
			}
		}

		if channelID == 0 {
			return "No possible channel :(", nil
		}

		invite, err := common.BotSession.ChannelInviteCreate(channelID, discordgo.Invite{
			MaxAge:    120,
			MaxUses:   1,
			Temporary: true,
			Unique:    true,
		})

		if err != nil {
			return nil, err
		}

		bot.SendDM(data.Author.ID, "discord.gg/"+invite.Code)
		return "Sent invite expiring in 120 seconds and with 1 use in DM", nil
	}),
}
