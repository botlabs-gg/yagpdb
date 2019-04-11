package topic

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Topic",
	Description: "Generates a conversation topic to help chat get moving.",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		doc, err := goquery.NewDocument("http://www.conversationstarters.com/generator.php")
		if err != nil {
			return nil, err
		}

		topic := doc.Find("#random").Text()
		return topic, nil
	},
}
