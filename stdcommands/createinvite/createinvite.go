package createinvite

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/stdcommands/util"
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
		{Name: "server", Type: dcmd.Int},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		gs := bot.State.Guild(true, data.Args[0].Int64())
		if gs == nil {
			return "Unknown server", nil
		}

		channelID := int64(0)
		gs.RLock()
		for _, v := range gs.Channels {
			if channelID == 0 || v.Type != discordgo.ChannelTypeGuildVoice {
				channelID = v.ID
				if v.Type != discordgo.ChannelTypeGuildVoice {
					break
				}
			}
		}
		gs.RUnlock()

		if channelID == 0 {
			return "No possible channel :(", nil
		}

		invite, err := common.BotSession.ChannelInviteCreate(channelID, discordgo.Invite{
			MaxAge:  120,
			MaxUses: 1,
		})

		if err != nil {
			return "Failed creating invite", nil
		}

		bot.SendDM(data.Msg.Author.ID, "discord.gg/"+invite.Code)
		return "", nil
	}),
}
