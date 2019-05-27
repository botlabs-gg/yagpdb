package define

import (
	"fmt"

	"github.com/dpatrie/urbandictionary"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Define",
	Aliases:      []string{"df"},
	Description:  "Look up an urban dictionary definition",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Topic", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		qResp, err := urbandictionary.Query(data.Args[0].Str())
		if err != nil {
			return "Failed querying :(", err
		}

		if len(qResp.Results) < 1 {
			return "No result :(", nil
		}

		result := qResp.Results[0]

		cmdResp := fmt.Sprintf("**%s**: %s\n*%s*\n*(<%s>)*", result.Word, result.Definition, result.Example, result.Permalink)
		if len(qResp.Results) > 1 {
			cmdResp += fmt.Sprintf(" *%d more results*", len(qResp.Results)-1)
		}

		return cmdResp, nil
	},
}
