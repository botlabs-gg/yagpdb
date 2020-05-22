package poll

import (
	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var (
	pollReactions = [...]string{"1⃣", "2⃣", "3⃣", "4⃣", "5⃣", "6⃣", "7⃣", "8⃣", "9⃣", "🔟"}
	Command       = &commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "Poll",
		Description:  "Create very simple reaction poll. Example: `poll \"favorite color?\" blue red pink`",
		RequiredArgs: 3,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{
				Name: "Topic",
				Type: dcmd.String,
				Help: "Description of the poll",
			},
			&dcmd.ArgDef{Name: "Option1", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option2", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option3", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option4", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option5", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option6", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option7", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option8", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option9", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Option10", Type: dcmd.String},
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

	authorName := data.MS.Nick
	if authorName == "" {
		authorName = data.MS.Username
	}

	response := discordgo.MessageEmbed{
		Title:       topic,
		Description: description,
		Color:       0x65f442,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: discordgo.EndpointUserAvatar(data.MS.ID, data.Msg.Author.Avatar),
		},
	}

	common.BotSession.ChannelMessageDelete(data.Msg.ChannelID, data.Msg.ID)
	pollMsg, err := common.BotSession.ChannelMessageSendEmbed(data.Msg.ChannelID, &response)
	if err != nil {
		return nil, errors.WrapIf(err, "failed to add poll description")
	}
	for i, _ := range options {
		common.BotSession.MessageReactionAdd(pollMsg.ChannelID, pollMsg.ID, pollReactions[i])
	}
	return nil, nil
}
