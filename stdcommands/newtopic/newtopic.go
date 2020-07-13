package newtopic

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "NewTopic",
	Description: "Generates a conversation topic to help chat get moving.",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		topic := ""
		topic = fmt.Sprintf("**%s**", randomTopic())
		return topic, nil
	},
}