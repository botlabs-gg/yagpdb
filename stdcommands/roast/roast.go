package roast

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/lunixbochs/vtclean"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Roast",
	Aliases:     []string{"insult"},
	Description: "sends roasts from EvilInsult.com",
	Arguments: []*dcmd.ArgDef{
		{Name: "Target", Type: dcmd.User},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	NSFW:                true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		target := "a random person nearby"
		if data.Args[0].Value != nil {
			target = data.Args[0].Value.(*discordgo.User).Username
		}
		req, err := http.NewRequest("GET", "https://evilinsult.com/generate_insult.php?lang=en", nil)
		if err != nil {
			return "Not enough fire to roast", err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		insult := vtclean.Clean(string(body), false)
		embed := &discordgo.MessageEmbed{}
		embed.Title = fmt.Sprintf(`%s roasted %s`, data.Author.Username, target)
		embed.Description = insult
		return embed, nil
	},
}
