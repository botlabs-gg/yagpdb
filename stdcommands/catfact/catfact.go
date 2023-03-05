package catfact

import (
	"math/rand"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "CatFact",
	Aliases:             []string{"cf", "cat", "catfacts"},
	Description:         "Cat Facts",
	DefaultEnabled:      true,
	SlashCommandEnabled: true,

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		cf := Catfacts[rand.Intn(len(Catfacts))]
		return cf, nil
	},
}
