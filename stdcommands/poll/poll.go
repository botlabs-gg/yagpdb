package poll

import (
	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var (
	pollReactions = [...]string{"1⃣", "2⃣", "3⃣", "4⃣", "5⃣", "6⃣", "7⃣", "8⃣", "9⃣", "🔟"}
	Command       = &commands.YAGCommand{
		CmdCategory:         commands.CategoryTool,
		Name:                "Poll",
		Description:         "Create very simple reaction poll. Example: `poll \"favorite color?\" blue red pink`",
		RequiredArgs:        3,
		SlashCommandEnabled: true,
		Arguments: []*dcmd.ArgDef{
			{
				Name: "Topic",
				Type: dcmd.String,
				Help: "Description of the poll",
			},
			{Name: "Option1", Type: dcmd.String},
			{Name: "Option2", Type: dcmd.String},
			{Name: "Option3", Type: dcmd.String},
			{Name: "Option4", Type: dcmd.String},
			{Name: "Option5", Type: dcmd.String},
			{Name: "Option6", Type: dcmd.String},
			{Name: "Option7", Type: dcmd.String},
			{Name: "Option8", Type: dcmd.String},
			{Name: "Option9", Type: dcmd.String},
			{Name: "Option10", Type: dcmd.String},
		},
		RunFunc: createPoll,
	}
)

func createPoll(data *dcmd.Data) (interface{}, error) {
	topic := data.Args[0].Str()
	options := data.Args[1:]
	for i, option := range options {
		if option.Str() == "" || i >= len(pollReactions) {
			options = options[:i]
			break
		}
	}

	var description string
	for i, option := range options {
		if i != 0 {
			description += "\n"
		}
		description += pollReactions[i] + " " + option.Str()
	}

	authorName := data.GuildData.MS.Member.Nick
	if authorName == "" {
		authorName = data.GuildData.MS.User.Username
	}

	response := discordgo.MessageEmbed{
		Title:       topic,
		Description: description,
		Color:       0x65f442,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: discordgo.EndpointUserAvatar(data.GuildData.MS.User.ID, data.Author.Avatar),
		},
	}

	if data.TraditionalTriggerData != nil {
		common.BotSession.ChannelMessageDelete(data.ChannelID, data.TraditionalTriggerData.Message.ID)
	}

	pollMsg, err := common.BotSession.ChannelMessageSendEmbed(data.ChannelID, &response)
	if err != nil {
		return nil, errors.WrapIf(err, "failed to add poll description")
	}
	for i := range options {
		common.BotSession.MessageReactionAdd(pollMsg.ChannelID, pollMsg.ID, pollReactions[i])
	}
	return nil, nil
}
