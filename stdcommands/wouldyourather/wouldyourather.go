package wouldyourather

import (
	"fmt"
	"math/rand"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type WouldYouRather struct {
	OptionA string
	OptionB string
}

func randomQuestion() WouldYouRather {
	return Questions[rand.Intn(len(Questions))]
}

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "WouldYouRather",
	Aliases:     []string{"wyr"},
	Description: "Get presented with 2 options.",
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "raw", Help: "Raw output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		question := randomQuestion()

		wyrDescription := fmt.Sprintf("**EITHER...**\n🇦 %s\n\n **OR...**\n🇧 %s", question.OptionA, question.OptionB)

		if data.Switches["raw"].Value != nil && data.Switches["raw"].Value.(bool) {
			return wyrDescription, nil
		}

		embed := &discordgo.MessageEmbed{
			Description: wyrDescription,
			Author: &discordgo.MessageEmbedAuthor{
				Name: "Would you rather...",
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Requested by: %s", data.Author.String()),
			},
			Color: rand.Intn(16777215),
		}

		msg, err := common.BotSession.ChannelMessageSendEmbed(data.ChannelID, embed)
		if err != nil {
			return nil, err
		}

		common.BotSession.MessageReactionAdd(data.ChannelID, msg.ID, "🇦")
		err = common.BotSession.MessageReactionAdd(data.ChannelID, msg.ID, "🇧")
		if err != nil {
			return nil, err
		}

		return nil, nil
	},
}