package dadjoke

import (
	"encoding/json"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

type Joke struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

var Command = &commands.YAGCommand{
	Cooldown:            5,
	CmdCategory:         commands.CategoryFun,
	Name:                "DadJoke",
	Description:         "Generates a dad joke using the API from icanhazdadjoke.",
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (any, error) {
		header := http.Header{"Accept": []string{"application/json"}}
		_, bytes, err := util.HTTPGet(data.Context(), "https://icanhazdadjoke.com", header)
		if err != nil {
			return nil, err
		}
		var joke Joke
		err = json.Unmarshal(bytes, &joke)
		if err != nil {
			return nil, err
		}
		//Return the joke - the other pieces are unneeded and ignored.
		resp := joke.Joke
		return resp, nil
	},
}
