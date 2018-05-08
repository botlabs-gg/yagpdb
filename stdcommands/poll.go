package stdcommands

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
)

var (
	pollReactions = [...]string{"1⃣", "2⃣", "3⃣", "4⃣", "5⃣", "6⃣", "7⃣", "8⃣", "9⃣", "🔟"}
	cmdPoll       = &commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "Poll",
		Description:  "Create a reaction poll.",
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

	author := data.Msg.Author
	authorName := author.Username
	if member, err := bot.GetMember(data.GS.ID(), author.ID); err == nil {
		authorName = member.Nick
	}

	response := discordgo.MessageEmbed{
		Title:       topic,
		Description: description,
		Color:       0x65f442,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: discordgo.EndpointUserAvatar(author.ID, author.Avatar),
		},
	}

	common.BotSession.ChannelMessageDelete(data.Msg.ChannelID, data.Msg.ID)
	pollMsg, err := common.BotSession.ChannelMessageSendEmbed(data.Msg.ChannelID, &response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add poll description")
	}
	for i, _ := range options {
		common.BotSession.MessageReactionAdd(pollMsg.ChannelID, pollMsg.ID, pollReactions[i])
	}
	return nil, nil
}
