package howlongtobeat

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type hltb struct {
	GameTitle               string `json:"game_name"`
	GameID                  int64  `json:"game_id"`
	ImagePath               string `json:"game_image"`
	MainStoryTime           int64  `json:"comp_main"`
	MainStoryTimeHours      string
	MainExtraTime           int64 `json:"comp_plus"`
	MainStoryExtraTimeHours string
	CompletionistTime       int64 `json:"comp_100"`
	CompletionistTimeHours  string
	GameURL                 string
	ImageURL                string
	JaroWinklerSimilarity   float64
}

type hltbRequest struct {
	SearchType  string   `json:"searchType"`
	SearchTerms []string `json:"searchTerms"`
}

var (
	hltbScheme   = "https"
	hltbHost     = "howlongtobeat.com"
	hltbHostPath = "api/search"
)

//Command var needs a comment for lint :)
var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Name:                "HowLongToBeat",
	Aliases:             []string{"hltb"},
	RequiredArgs:        1,
	Description:         "Game information based on query from howlongtobeat.com.\nResults are sorted by popularity, it's their default. Without -p returns the first result.\nSwitch -p gives paginated output using the Jaro-Winkler similarity metric sorting max 20 results.",
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "Game-Title", Type: dcmd.String},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "c", Help: "Compact output"},
		{Name: "p", Help: "Paginated output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var compactView, paginatedView bool
		gameName := data.Args[0].Str()

		if data.Switches["c"].Value != nil && data.Switches["c"].Value.(bool) {
			compactView = true
		}

		if data.Switches["p"].Value != nil && data.Switches["p"].Value.(bool) {
			compactView = false
			paginatedView = true
		}

		games, err := getGameData(gameName)
		if err != nil {
			return nil, err
		}

		if len(games) == 0 {
			return "No results", nil
		}

		games = parseGameData(gameName, games)
		if compactView {
			compactData := fmt.Sprintf("%s: %s | %s | %s | <%s>",
				normaliseTitle(games[0].GameTitle),
				games[0].MainStoryTimeHours,
				games[0].MainStoryExtraTimeHours,
				games[0].CompletionistTimeHours,
				games[0].GameURL,
			)
			return compactData, nil
		}

		hltbEmbed := embedCreator(games, 0, paginatedView)

		if paginatedView {
			_, err := paginatedmessages.CreatePaginatedMessage(
				data.GuildData.GS.ID, data.ChannelID, 1, len(games), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					i := page - 1
					sort.SliceStable(games, func(i, j int) bool {
						return games[i].JaroWinklerSimilarity > games[j].JaroWinklerSimilarity
					})
					paginatedEmbed := embedCreator(games, i, paginatedView)
					return paginatedEmbed, nil
				})
			if err != nil {
				return "Something went wrong", nil
			}
		} else {
			return hltbEmbed, nil
		}

		return nil, nil
	},
}

func embedCreator(games []hltb, i int, paginated bool) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: normaliseTitle(games[i].GameTitle),
			URL:  games[i].GameURL,
		},

		Color: int(rand.Int63n(16777215)),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: games[i].ImageURL,
		},
	}
	if games[i].MainStoryTime > 0 {
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{Name: "Main Story", Value: games[i].MainStoryTimeHours})
	}
	if games[i].MainExtraTime > 0 {
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{Name: "Main + Extra", Value: games[i].MainStoryExtraTimeHours})
	}
	if games[i].CompletionistTime > 0 {
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{Name: "Completionist", Value: games[i].CompletionistTimeHours})
	}
	if paginated {
		embed.Description = fmt.Sprintf("Term similarity: %.1f%%", games[i].JaroWinklerSimilarity*100)
	}

	return embed
}

func normaliseTitle(t string) string {
	return strings.Join(strings.Fields(t), " ")
}
