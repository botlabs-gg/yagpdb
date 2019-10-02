package catfact

import (
	"math/rand"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "CatFact",
	Aliases:     []string{"cf", "cat", "catfacts"},
	Description: "Cat Facts",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		cf := Catfacts[rand.Intn(len(Catfacts))]
		return cf, nil
	},
}
