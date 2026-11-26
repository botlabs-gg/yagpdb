package inspire

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "Inspire",
	Aliases:             []string{"insp"},
	Description:         "Shows 'inspirational' quotes from inspirobot.me",
	RunInDM:             false,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	Cooldown:            3,
	Arguments: []*dcmd.ArgDef{
		{Name: "Season", Type: dcmd.String, Default: ""},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "mindfulness", Help: "Generates Mindful Quotes!"},
	},
	RunFunc: func(data *dcmd.Data) (any, error) {
		available := map[string]bool{"xmas": true}
		inspireArray := []string{}
		var paginatedView bool
		season := strings.ToLower(data.Args[0].Str())
		if len(season) > 0 && !available[season] {
			return "Available seasons: `xmas`", nil
		}
		paginatedView = false
		if data.Switches["mindfulness"].Value != nil && data.Switches["mindfulness"].Value.(bool) {
			paginatedView = true
		}
		if paginatedView {
			result, err := inspireFromAPI(data.Context(), true, season)
			if err != nil {
				return nil, err
			}
			inspireArray = arrayMaker(inspireArray, result)
			return paginatedmessages.NewPaginatedResponse(
				data.GuildData.GS.ID, data.ChannelID, 1, 15, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					if page-1 == len(inspireArray) {
						// page changes happen after the command context has ended
						result, err := inspireFromAPI(context.Background(), true, season)
						if err != nil {
							return nil, err
						}
						inspireArray = arrayMaker(inspireArray, result)
					}
					return createInspireEmbed(inspireArray[page-1], true), nil
				}), nil
		}
		inspData, err := inspireFromAPI(data.Context(), false, season)
		if err != nil {
			return fmt.Sprintf("%s\nInspiroBot wonky... sad times :/", err), err
		}
		embed := createInspireEmbed(inspData, false)
		return embed, nil
	},
}

func inspireFromAPI(ctx context.Context, mindfulnessMode bool, season string) (string, error) {
	query := "https://inspirobot.me/api?generate=true"
	if mindfulnessMode {
		query = fmt.Sprintf("https://inspirobot.me/api?generateFlow=1&sessionID=%d", time.Now().UTC().Unix())
	} else if len(season) > 0 {
		query = "https://inspirobot.me/api?generate=true&season=" + season
	}

	status, body, err := util.HTTPGet(ctx, query, nil)
	if err != nil {
		return "", err
	}

	if status != http.StatusOK {
		return "", commands.NewPublicError("HTTP err: ", status)
	}

	if mindfulnessMode {
		var mindful MindfulnessMode

		err := json.Unmarshal(body, &mindful)
		if err != nil {
			return "", err
		}
		if len(mindful.Data) < 2 {
			return "", commands.NewPublicError("InspiroBot returned no quote")
		}

		mindfulness := mindful.Data[1].Text
		if len(mindfulness) > 4000 {
			mindfulness = common.CutStringShort(mindfulness, 4000)
		}

		return mindfulness, nil
	}

	return string(body), nil
}

func createInspireEmbed(data string, mindfulness bool) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}

	if mindfulness {
		embed.Title = "Here's an inspirational quote (Mindfulness Mode):"
		embed.Description = "```\n" + data + "```"
		embed.Color = int(11413503)
	} else {
		embed.Color = int(rand.Int63n(0xffffff))
		embed.Description = "Here's an inspirational quote:"
		embed.Image = &discordgo.MessageEmbedImage{
			URL: data,
		}
	}
	return embed
}
func arrayMaker(list []string, data string) []string {
	if data != "" {
		re := regexp.MustCompile(`\[pause \d+\]`)
		list = append(list, re.ReplaceAllString(data, ""))
	}
	return list
}

type MindfulnessMode struct {
	Data []struct {
		Duration *float64 `json:"duration,omitempty"`
		Image    *string  `json:"image,omitempty"`
		Type     *string  `json:"type"`
		Time     *float64 `json:"time"`
		Text     string   `json:"text,omitempty"`
	} `json:"data"`
}
