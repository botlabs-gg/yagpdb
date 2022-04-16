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
	JOKE string `json:"joke"`
	STATUS int `json:"status"`
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

		client := &http.Client{}
		apiResp, err := client.Do(req)
		if err != nil {
			return nil , err
		}
		//Once the rest of the function is done close our connection the API.
		defer apiResp.Body.Close()
		//Read the API response.
		bytes, err := ioutil.ReadAll(apiResp.Body)
		if err != nil {
			return nil , err
		}
		//Create our struct and unmarshal the content into it.
		joke1 := Joke{}
		jsonErr := json.Unmarshal(bytes,&joke1)
		if jsonErr != nil {
			return nil , jsonErr
		}
		//Return the joke - the other pieces are unneeded and ignored. 
		resp := joke1.JOKE
		return resp , nil
	},
}