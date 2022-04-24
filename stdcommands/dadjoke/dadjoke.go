package dadjoke

import (
	"io/ioutil"
	"net/http"
	"encoding/json"
	"github.com/jonas747/dcmd/v4"
	"github.com/botlabs-gg/yagpdb/commands"
)

//Create the struct that we will serialize the api respone into.
type Joke struct {
	ID string `json:"id"`
	Joke string `json:"joke"`
	Status int `json:"status"`
}

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "DadJoke",
	Description: "Generates a dad joke using the API from icanhazdadjoke.",
	DefaultEnabled: 		true,
	SlashCommandEnabled: 	true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		//Define the request and website we will navigate to. 
		req, err := http.NewRequest("GET", "https://icanhazdadjoke.com", nil)
		if err != nil {
			return nil , err
		}
		//Set the headers that will be sent to the API to determine the response.
		req.Header.Set("Accept", "application/json")
		req.Header.Add("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

		apiResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil , err
		}
		//Once the rest of the function is done close our connection the API.
		defer apiResp.Body.Close()
		//Read the API response.
		bytes, err := io.ReadAll(apiResp.Body)
		if err != nil {
			return nil , err
		}
		//Create our struct and unmarshal the content into it.
		var joke := Joke{}
		err := json.Unmarshal(bytes,&joke1)
		if err != nil {
			return nil , err
		}
		//Return the joke - the other pieces are unneeded and ignored. 
		return joke.Joke , nil
	},
}