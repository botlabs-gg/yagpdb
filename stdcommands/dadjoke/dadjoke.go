package dadjoke

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "DadJoke",
	Description: "Generates a dad joke for no reason other than annoying the staff.",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		doc, err := goquery.NewDocument("https://icanhazdadjoke.com/")
		if err != nil {
			return nil, err
		}

		topic := doc.Find("#subtitle").Text()
		return topic, nil
	},
}