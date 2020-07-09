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
		doc, err := goquery.NewDocument("https://capitalizemytitle.com/random-topic-generator/")
		if err != nil {
			return nil, err
		}

		topic := doc.Find("#blog-ideas-output").Text()
		return topic, nil
	},
}
