package topic

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Topic",
	Description: "Generates a conversation topic to help chat get moving.",
	Arguments: []*dcmd.ArgDef{
		{Name: "Target", Type: dcmd.User},
	},

	target := "a random person nearby"
	if data.Args[0].Value != nil {
		target = data.Args[0].Value.(*discordgo.User).Username
	}

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		topic := ""
		topic = fmt.Sprintf("Lets talk about **%s**", randomTopic(), target)
		return topic, nil
	},
}