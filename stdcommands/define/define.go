package define

import (
	"fmt"
	"math/rand"
	"net/url"
	"regexp"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/dpatrie/urbandictionary"
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Define",
	Aliases:      []string{"df"},
	Description:  "Look up an urban dictionary definition, default paginated view.",
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Topic", Type: dcmd.String},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "raw", Help: "Raw output"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var paginatedView bool
		paginatedView = true

		if data.Switches["raw"].Value != nil && data.Switches["raw"].Value.(bool) {
			paginatedView = false
		}

		qResp, err := urbandictionary.Query(data.Args[0].Str())
		if err != nil {
			return "Failed querying :(", err
		}

		if len(qResp.Results) < 1 {
			return "No result :(", nil
		}

		if paginatedView {
			_, err := paginatedmessages.CreatePaginatedMessage(
				data.GuildData.GS.ID, data.ChannelID, 1, len(qResp.Results), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					i := page - 1

					paginatedEmbed := embedCreator(qResp.Results, i)
					return paginatedEmbed, nil
				})
			if err != nil {
				return "Something went wrong", nil
			}
		} else {
			result := qResp.Results[0]

			cmdResp := fmt.Sprintf("**%s**: %s\n*%s*\n*(<%s>)*", result.Word, result.Definition, result.Example, result.Permalink)
			if len(qResp.Results) > 1 {
				cmdResp += fmt.Sprintf(" *%d more results*", len(qResp.Results)-1)
			}
			return cmdResp, nil
		}

		return nil, nil
	},
}

func embedCreator(udResult []urbandictionary.Result, i int) *discordgo.MessageEmbed {
	definition := udResult[i].Definition
	if len(definition) > 2000 {
		definition = common.CutStringShort(definition, 2000) + "\n\n(definition too long)"
	}
	example := "None given"
	if len(udResult[i].Example) > 0 {
		example = linkReferencedTerms(udResult[i].Example)
	}
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: udResult[i].Word,
			URL:  udResult[i].Permalink,
		},
		Description: fmt.Sprintf("**Definition**: %s", linkReferencedTerms(definition)),
		Color:       int(rand.Int63n(16777215)),
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{Name: "Example:", Value: example},
			&discordgo.MessageEmbedField{Name: "Author:", Value: udResult[i].Author},
			&discordgo.MessageEmbedField{Name: "Votes:", Value: fmt.Sprintf("Upvotes: %d\nDownvotes: %d", udResult[i].Upvote, udResult[i].Downvote)},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://upload.wikimedia.org/wikipedia/commons/thumb/8/82/UD_logo-01.svg/512px-UD_logo-01.svg.png",
		},
	}

	return embed
}

const urbanDictionaryDefineEndpoint = "https://www.urbandictionary.com/define.php?term="

var termRefRe = regexp.MustCompile(`\[.+?\]`)

func linkReferencedTerms(def string) string {
	return termRefRe.ReplaceAllStringFunc(def, func(match string) string {
		term := match[1 : len(match)-1]
		return fmt.Sprintf("[%s](%s%s)", term, urbanDictionaryDefineEndpoint, url.QueryEscape(term))
	})
}
