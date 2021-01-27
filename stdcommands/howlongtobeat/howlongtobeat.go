package howlongtobeat

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/commands"
)

type hltb struct {
	GameTitle     string
	PureTitle     string //useful for Levenshtein calculation, symbol clutter removed
	GameURL       string
	ImageURL      string
	MainStory     []string
	MainExtra     []string
	Completionist []string
	LevDistance   int
	LevSimilarity float64
	OnlineGame    bool
}

var (
	hltbScheme   = "https"
	hltbHost     = "howlongtobeat.com"
	hltbURL      = fmt.Sprintf("%s://%s/", hltbScheme, hltbHost)
	hltbHostPath = "search_results.php"
	hltbRawQuery = "page=1"
)

//Command var needs a comment for lint :)
var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "HowLongToBeat",
	Aliases:      []string{"hltb"},
	RequiredArgs: 1,
	Description:  "Game information based on query from howlongtobeat.com.\nResults are sorted by popularity, it's their default. Without -p returns the first result.\nSwitch -p gives paginated output using Levenshtein distance sorting max 20 results.",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Game title", Type: dcmd.String},
	},
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "c", Name: "Compact output"},
		&dcmd.ArgDef{Switch: "p", Name: "Paginated output"},
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

		getData, err := getGameData(gameName)
		if err != nil {
			return nil, err
		}
		toReader := strings.NewReader(getData)

		hltbQuery, err := parseGameData(gameName, toReader)
		if err != nil {
			return nil, err
		}

		if hltbQuery[0].GameTitle == "" {
			return "No results", nil
		}

		if compactView {
			compactData := fmt.Sprintf("%s: %s | %s | %s | <%s>",
				hltbQuery[0].GameTitle,
				strings.Trim(fmt.Sprint(hltbQuery[0].MainStory), "[]"),
				strings.Trim(fmt.Sprint(hltbQuery[0].MainExtra), "[]"),
				strings.Trim(fmt.Sprint(hltbQuery[0].Completionist), "[]"),
				hltbQuery[0].GameURL)
			return compactData, nil
		}

		hltbEmbed := embedCreator(hltbQuery, 0, paginatedView)

		if paginatedView {
			_, err := paginatedmessages.CreatePaginatedMessage(
				data.GS.ID, data.CS.ID, 1, len(hltbQuery), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					i := page - 1
					sort.SliceStable(hltbQuery, func(i, j int) bool {
						return hltbQuery[i].LevDistance < hltbQuery[j].LevDistance
					})
					paginatedEmbed := embedCreator(hltbQuery, i, paginatedView)
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

func embedCreator(hltbQuery []hltb, i int, paginated bool) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: hltbQuery[i].GameTitle,
			URL:  hltbQuery[i].GameURL,
		},

		Color: int(rand.Int63n(16777215)),
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: hltbQuery[i].MainStory[0], Value: hltbQuery[i].MainStory[1]},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: hltbQuery[i].ImageURL,
		},
	}
	if len(hltbQuery[i].MainExtra) > 0 {
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{Name: hltbQuery[i].MainExtra[0], Value: hltbQuery[i].MainExtra[1]})
	}
	if len(hltbQuery[i].Completionist) > 0 {
		embed.Fields = append(embed.Fields,
			&discordgo.MessageEmbedField{Name: hltbQuery[i].Completionist[0], Value: hltbQuery[i].Completionist[1]})
	}
	if paginated {
		embed.Description = fmt.Sprintf("Lev distance: %d\nTerm similarity: %.1f%%", hltbQuery[i].LevDistance, hltbQuery[i].LevSimilarity*100)
	}
	return embed
}
