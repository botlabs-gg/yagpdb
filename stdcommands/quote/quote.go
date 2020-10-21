package quote
 
import (
		"encoding/json"
		"io/ioutil"
		"math/rand"
		"time"
 
		"github.com/jonas747/dcmd"
		"github.com/jonas747/yagpdb/commands"
)
 
var Command = &commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Name:        "Quote",
		Aliases:     []string{"quote", "saying"},
		Description: "Random Quotes",
 
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
				var quotes Quotes
 
				byteValue, _ := ioutil.ReadFile("./quotes.json")
				json.Unmarshal(byteValue, &quotes)
 
				idx := rand.Intn(len(data))
				q := fmt.Sprintf("%s\n~ %s", quotes[idx].Text, quotes[idx].Author)
				return q, nil
		}
}
 
type Quotes []struct {
		Author string `json:"author"`
		Text   string `json:"text"`
		Source string `json:"source,omitempty"`
		Tags   string `json:"tags,omitempty"`
}