package dogfact

import (
	"math/rand"

	"github.com/jonas747/dcmd/v2"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "DogFact",
	Aliases:             []string{"dog", "dogfacts"},
	Description:         "Dog Facts",
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		df := dogfacts[rand.Intn(len(dogfacts))]
		return df, nil
	},
}
