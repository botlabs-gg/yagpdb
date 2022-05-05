package roll

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/dice"
)

var Command = &commands.YAGCommand{
	CmdCategory:     commands.CategoryFun,
	Name:            "Roll",
	Description:     "Roll dices, specify nothing for 6 sides, specify a number for max sides, or rpg dice syntax.",
	LongDescription: "Example: `-roll 2d6`",
	Arguments: []*dcmd.ArgDef{
		{Name: "Sides", Default: 0, Type: dcmd.Int},
		{Name: "RPG-Dice", Type: dcmd.String},
	},
	ArgumentCombos:      [][]int{{0}, {1}, {}},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		if data.Args[1].Value != nil {
			// Special dice syntax if string
			r, _, err := dice.Roll(strings.ToLower(data.Args[1].Str()))
			if err != nil {
				return err.Error(), nil
			}

			output := r.String()
			if len(output) > 100 {
				output = output[:100] + "..."
			} else {
				output = strings.TrimSuffix(output, "([])")
			}

			return ":game_die: " + output, nil
		}

		// normal, n sides dice rolling
		sides := data.Args[0].Int()
		if sides < 1 {
			sides = 6
		}

		result := rand.Intn(sides)
		output := fmt.Sprintf(":game_die: %d (1 - %d)", result+1, sides)
		return output, nil
	},
}
