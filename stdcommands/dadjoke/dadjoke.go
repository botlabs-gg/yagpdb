package dadjoke

import (
	"io/ioutil"
	"log"
	"net/http"
	"encoding/json"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

type Joke struct {
	ID string `json:"id"`
	JOKE string `json:"joke"`
	STATUS int `json:"status"`
}

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "DadJoke",
	Description: "Generates a dad joke for no reason other than annoying the staff.",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		req, err := http.NewRequest("GET", "https://icanhazdadjoke.com", nil)
		if err != nil {
			log.Fatalln(err)
		}
	
		req.Header.Set("Accept", "application/json")
	
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalln(err)
		}
	
		defer resp.Body.Close()
	
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		joke1 := Joke{}
		jsonErr := json.Unmarshal(bytes,&joke1)
		if jsonErr != nil {
			log.Fatal(jsonErr)
		}
		response := joke1.JOKE
		return response  , nil
	},
}