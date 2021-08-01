package topic

import (
	"github.com/jonas747/dcmd/v3"
	"github.com/jonas747/yagpdb/commands"
	"math/rand"
)

var Command = &commands.YAGCommand{
	Cooldown:            5,
	CmdCategory:         commands.CategoryFun,
	Name:                "Topic",
	Description:         "Generates a conversation topic to help chat get moving.",
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		topic := ChatTopics[rand.Intn(len(ChatTopics))]
		return "> " + topic, nil
	},
}
