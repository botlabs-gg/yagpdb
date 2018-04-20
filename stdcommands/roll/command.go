package roll

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dice"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"math/rand"
)

var yagCommand = commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Roll",
	Description: "Roll dices, specify nothing for 6 sides, specify a number for max sides, or rpg dice syntax",
	Arguments: []*dcmd.ArgDef{
		{Name: "RPG Dice", Type: dcmd.String},
		{Name: "Sides", Default: 0, Type: dcmd.Int},
	},
	ArgumentCombos: [][]int{[]int{1}, []int{0}, []int{}},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		if data.Args[0].Value != nil {
			// Special dice syntax if string
			r, _, err := dice.Roll(data.Args[0].Str())
			if err != nil {
				return err.Error(), nil
			}
			return r.String(), nil
		}

		// normal, n sides dice rolling
		sides := data.Args[1].Int()
		if sides < 1 {
			sides = 6
		}

		result := rand.Intn(sides)
		return fmt.Sprintf(":game_die: %d (1 - %d)", result+1, sides), nil
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
