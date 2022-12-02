package wouldyourather

import (
	"fmt"
	"math/rand"
	"net/http"

	"emperror.dev/errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "WouldYouRather",
	Aliases:     []string{"wyr"},
	Description: "Get presented with 2 options.",
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "raw", Help: "Raw output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		q1, q2, err := wouldYouRather()
		if err != nil {
			return nil, err
		}

		wyrDescription := fmt.Sprintf("**EITHER...**\n🇦 %s\n\n **OR...**\n🇧 %s", q1, q2)

		if data.Switches["raw"].Value != nil && data.Switches["raw"].Value.(bool) {
			return wyrDescription, nil
		}

		embed := &discordgo.MessageEmbed{
			Description: wyrDescription,
			Author: &discordgo.MessageEmbedAuthor{
				Name: "Would you rather...",
				URL:  "https://either.io/",
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Requested by: %s#%s", data.Author.Username, data.Author.Discriminator),
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

func wouldYouRather() (q1 string, q2 string, err error) {
	req, err := http.NewRequest("GET", "http://either.io/", nil)
	if err != nil {
		panic(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	r1 := doc.Find("div.result.result-1 > .option-text")
	r2 := doc.Find("div.result.result-2 > .option-text")

	if len(r1.Nodes) < 1 || len(r2.Nodes) < 1 {
		return "", "", errors.New("Failed finding questions, format may have changed.")
	}

	q1 = r1.Nodes[0].FirstChild.Data
	q2 = r2.Nodes[0].FirstChild.Data
	return
}
