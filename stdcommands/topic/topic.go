package topic


import (
	"fmt"
	"math/rand"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Topic",
	Description: "Generates a conversation topic to help chat get moving.",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		resp := ""
		resp = fmt.Sprintf("Lets talk about **%s**", randomTopic())
		return resp, nil
	},
}