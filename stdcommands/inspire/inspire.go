package inspire

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "Inspire",
	Aliases:             []string{"insp"},
	Description:         "Shows 'inspirational' quotes from inspirobot.me...",
	RunInDM:             true,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		inspireURL, err := inspireFromAPI()
		if err != nil {
			return fmt.Sprintf("%s\nInspiroBot wonky... sad times :/", err), err
		}

		embed := &discordgo.MessageEmbed{
			Description: "Here's an inspirational quote:",
			Color:       int(rand.Int63n(0xffffff)),
			Image: &discordgo.MessageEmbedImage{
				URL: inspireURL,
			},
		}

		return embed, nil
	},
}

func inspireFromAPI() (string, error) {
	query := "https://inspirobot.me/api?generate=true"
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "curl/7.83.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", commands.NewPublicError("HTTP err: ", resp.StatusCode)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	inspireReturn := string(body)

	return inspireReturn, nil
}
