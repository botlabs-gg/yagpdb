package songwhip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/jonas747/dcmd/v4"
	"github.com/jonas747/discordgo/v2"
	"github.com/jonas747/dstate/v4"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

type Artist struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Songwhip struct {
	Url     string    `json:"url"`
	Image   string    `json:"image"`
	Name    string    `json:"name"`
	Artists []*Artist `json:"artists"`
}

var Command = &commands.YAGCommand{
	Cooldown:            15,
	SlashCommandEnabled: true,
	CmdCategory:         commands.CategoryFun,
	Name:                "Songwhip",
	Description:         "Create a songwhip link, to share your music regardless of the platform!",
	Aliases:             []string{"sw"},
	RequiredArgs:        1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Link", Type: dcmd.String},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		songURL := data.Args[0].Str()

		// Check if it is a valid URI to be easy on the API and return early
		_, err := url.ParseRequestURI(songURL)
		if err != nil {
			return fmt.Sprintf("Not a valid URL!\n`%s`", err), err
		}

		songwhip, err := getSongwhip(songURL)
		if err != nil {
			return "Something went wrong with getting the page link! Perhaps an invalid song URL?", err
		}

		embed := makeEmbed(songwhip, data.GuildData.MS)

		return embed, nil
	},
}

func getSongwhip(url string) (*Songwhip, error) {
	songwhip := Songwhip{}
	payload := map[string]string{"url": url}
	jsonValue, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Post("https://songwhip.com", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&songwhip)
	if err != nil {
		return nil, err
	}

	return &songwhip, nil
}

func makeEmbed(s *Songwhip, ms *dstate.MemberState) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		URL: s.Url,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Songwhip • Listen on any platform",
			IconURL: "https://songwhip.com/apple-touch-icon.png",
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("Requested by: %s#%s", ms.User.Username, ms.User.Discriminator),
			IconURL: discordgo.EndpointUserAvatar(ms.User.ID, ms.User.Avatar),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: s.Image,
		},
		Title:       fmt.Sprintf("%s by %s", s.Name, s.Artists[0].Name),
		Description: common.CutStringShort(s.Artists[0].Description, 280),
		Color:       int(rand.Int63n(16777215)),
	}

	return embed
}
