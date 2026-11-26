package calc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/ei14/calc/compute"
)

var (
	// calc/compute isnt threadsafe :'(
	computeLock sync.Mutex
)

var replacer = strings.NewReplacer("x", "*", "×", "*", "÷", "/", "++", "+", "--", "- -")

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryTool,
	Name:         "Calc",
	Aliases:      []string{"c", "calculate"},
	Description:  "Calculator 2+2=5",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Expression", Type: dcmd.String},
	},
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (any, error) {
		computeLock.Lock()
		defer computeLock.Unlock()
		toCompute := data.Args[0].Str()
		toCompute = replacer.Replace(toCompute)

		// the library keeps a global result history that @ reads from, so it would grow forever
		// and expose other users' results
		compute.ClearHistory()
		result, err := compute.Evaluate(toCompute)
		compute.ClearHistory()
		if err != nil {
			return "Failed evaluating the expression.", nil
		}

		return fmt.Sprintf("Result: `%f`", result), nil
	},
}
