package deletedm

import (
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	Name:                 "deletedm",
	Description:          "",
	Cooldown:             10,
	CmdCategory:          commands.CategoryGeneral,
	RunInDM:              true,
	MessageCommand:       true,
	IsResponseEphemeral:  true,
	DefaultEnabled:       false,
	HideFromCommandsPage: true,

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		if data.SlashCommandTriggerData.Interaction.Type != discordgo.InteractionApplicationCommand {
			return "Something went wrong!", nil
		}
		if data.SlashCommandTriggerData.Interaction.GuildID != 0 {
			return "This can only be used in DMs!", nil
		}

		i := data.SlashCommandTriggerData.Interaction

		channel, err := common.BotSession.UserChannelCreate(i.User.ID)
		if err != nil {
			return nil, err
		}

		targetMessageId := i.ApplicationCommandData().TargetID
		targetMessage, err := common.BotSession.ChannelMessage(channel.ID, targetMessageId)
		if err != nil {
			return nil, err
		}
		if targetMessage.Author.ID != common.BotApplication.ID {
			return "You can only use this on YAGPDB messages!", nil
		}

		if err := common.BotSession.ChannelMessageDelete(channel.ID, targetMessageId); err != nil {
			return nil, err
		}

		return "Successfully deleted message!", nil
	},
}
