package dadjoke

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
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
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		req, err := http.NewRequest("GET", "https://icanhazdadjoke.com", nil)
		if err != nil {
			return nil, err
		}
		//Set the headers that will be sent to the API to determine the response.
		req.Header.Set("Accept", "application/json")
		req.Header.Add("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

		apiResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer apiResp.Body.Close()
		bytes, err := io.ReadAll(apiResp.Body)
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
