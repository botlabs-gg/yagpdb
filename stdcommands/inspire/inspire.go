package inspire

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "Inspire",
	Aliases:             []string{"insp"},
	Description:         "Shows 'inspirational' quotes from inspirobot.me...",
	RunInDM:             false,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	Cooldown:            3,
	Arguments: []*dcmd.ArgDef{
		{Name: "Mindfulness", Type: &dcmd.IntArg{Min: 1, Max: 100}},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		query := "https://inspirobot.me/api?generate=true"

		if data.Args[0].Str() != "" {
			//Generate Unique Session ID
			ID, err := inspireFromAPI("https://inspirobot.me/api?getSessionID=1")
			if err != nil {
				return nil, err
			}
			//Use Unique Session ID for query
			query = fmt.Sprint("https://inspirobot.me/api?generateFlow=1&sessionID=", ID)
			result, err := inspireFromAPIMf(query)
			if err != nil {
				return nil, err
			}
			inspireArray := []string{}
			inspireArray = arrayMaker(inspireArray, result.Data)
			//Pagination decided max pages by user input API gives only 3 at a time :/
			_, err = paginatedmessages.CreatePaginatedMessage(
				data.GuildData.GS.ID, data.ChannelID, 1, data.Args[0].Int(), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					if page-1 == len(inspireArray) {
						result, err := inspireFromAPIMf(query)
						if err != nil {
							return nil, err
						}
						inspireArray = arrayMaker(inspireArray, result.Data)
					}
					return createInspireEmbed(inspireArray[page-1]), nil
				})
			if err != nil {
				return nil, nil
			}
			return "", nil
		} else {
			//Normal Image Inspire Output
			inspData, err := inspireFromAPI(query)
			if err != nil {
				return fmt.Sprintf("%s\nInspiroBot wonky... sad times :/", err), err
			}
			embed := &discordgo.MessageEmbed{
				Description: "Here's an inspirational quote:",
				Color:       int(rand.Int63n(0xffffff)),
				Image: &discordgo.MessageEmbedImage{
					URL: inspData,
				},
			}
			return embed, nil
		}
	},
}

func inspireFromAPI(query string) (string, error) {
	wonkyErr := "InspireAPI wonky... ducks are sad : /"
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return "wonkyErr", err
	}
	req.Header.Set("User-Agent", "curl/7.83.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return wonkyErr, err
	}
	if resp.StatusCode != 200 {
		return "", commands.NewPublicError("HTTP err: ", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wonkyErr, err
	}
	return string(body), nil
}
func inspireFromAPIMf(query string) (*RawObj, error) {
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "curl/7.83.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	rawObj := &RawObj{}
	err = json.Unmarshal([]byte(body), &rawObj)
	if err != nil {
		return nil, err
	}
	return rawObj, nil
}
func createInspireEmbed(data string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "Here's an inspirational quote (Mindfulness Mode):",
		Color:       int(11413503),
		Description: "```\n" + data + "```",
	}
	return embed
}
func arrayMaker(list []string, data []*InspireData) []string {
	for _, i := range data {
		if i.Text != "" {
			re := regexp.MustCompile(`\[pause \d+\]`)
			list = append(list, re.ReplaceAllString(i.Text, ""))
		}
	}
	return list
}

type RawObj struct {
	Data []*InspireData `json:"data"`
	Mp3  string         `json:"mp3"`
}
type InspireData struct {
	Duration *float64 `json:"duration,omitempty"`
	Image    *string  `json:"image,omitempty"`
	Type     *string  `json:"type"`
	Time     *float64 `json:"time"`
	Text     string   `json:"text,omitempty"`
}
