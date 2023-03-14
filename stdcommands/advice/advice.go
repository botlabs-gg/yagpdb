package advice

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "Advice",
	Description: "Don't be afraid to ask for advice!",
	Arguments: []*dcmd.ArgDef{
		{Name: "What", Type: dcmd.String},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		//return "The API this command used has been shut down :(", nil
		random := true
		addr := "http://api.adviceslip.com/advice"
		if data.Args[0].Str() != "" {
			random = false
			addr = "http://api.adviceslip.com/advice/search/" + url.QueryEscape(data.Args[0].Str())
		}

		resp, err := http.Get(addr)
		if err != nil {
			return nil, err
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
				advice = cast.Slips[rand.Intn(len(cast.Slips))].Advice
			}
		}

		return advice, nil
	},
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
