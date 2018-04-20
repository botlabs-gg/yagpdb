package topic

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var yagCommand = commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Topic",
	Description: "Generates a chat topic",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		doc, err := goquery.NewDocument("http://www.conversationstarters.com/generator.php")
		if err != nil {
			return err, err
		}

		topic := doc.Find("#random").Text()
		return topic, nil
	},
}

func Cmd() util.Command {
	return &cmd{}
}

type cmd struct {
	util.BaseCmd
}

func (c cmd) YAGCommand() *commands.YAGCommand {
	return &yagCommand
}
