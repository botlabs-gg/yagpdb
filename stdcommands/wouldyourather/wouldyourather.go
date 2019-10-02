package wouldyourather

import (
	"fmt"
	"net/http"

	"emperror.dev/errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "WouldYouRather",
	Aliases:     []string{"wyr"},
	Description: "Get presented with 2 options.",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		q1, q2, err := wouldYouRather()
		if err != nil {
			return nil, err
		}

		content := fmt.Sprintf("**Would you rather** (*<http://either.io>*)\n🇦 %s\n **OR**\n🇧 %s", q1, q2)
		msg, err := common.BotSession.ChannelMessageSend(data.Msg.ChannelID, content)
		if err != nil {
			return nil, err
		}

		common.BotSession.MessageReactionAdd(data.Msg.ChannelID, msg.ID, "🇦")
		err = common.BotSession.MessageReactionAdd(data.Msg.ChannelID, msg.ID, "🇧")
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

	doc, err := goquery.NewDocumentFromResponse(resp)
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
