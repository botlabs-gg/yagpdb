package dogfact

import (
	"math/rand"

	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/jonas747/dcmd/v4"
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
