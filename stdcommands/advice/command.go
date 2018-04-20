package advice

import (
	"encoding/json"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"net/http"
	"net/url"
)

var yagCommand = commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Advice",
	Description: "Get advice",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "What", Type: dcmd.String},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		random := true
		addr := "http://api.adviceslip.com/advice"
		if data.Args[0].Str() != "" {
			random = false
			addr = "http://api.adviceslip.com/advice/search/" + url.QueryEscape(data.Args[0].Str())
		}

		resp, err := http.Get(addr)
		if err != nil {
			return err, err
		}

		var decoded interface{}

		if random {
			decoded = &RandomAdviceResp{}
		} else {
			decoded = &SearchAdviceResp{}
		}

		err = json.NewDecoder(resp.Body).Decode(&decoded)
		if err != nil {
			return err, err
		}

		advice := "No advice found :'("

		if random {
			slip := decoded.(*RandomAdviceResp).Slip
			if slip != nil {
				advice = slip.Advice
			}
		} else {
			cast := decoded.(*SearchAdviceResp)
			if len(cast.Slips) > 0 {
				advice = cast.Slips[0].Advice
			}
		}

		return advice, nil
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

type AdviceSlip struct {
	Advice string `json:"advice"`
	ID     string `json:"slip_id"`
}

type RandomAdviceResp struct {
	Slip *AdviceSlip `json:"slip"`
}

type SearchAdviceResp struct {
	TotalResults json.Number   `json:"total_results"`
	Slips        []*AdviceSlip `json:"slips"`
}
