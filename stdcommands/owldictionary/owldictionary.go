package owldictionary

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/microcosm-cc/bluemonday"
)

var confOwlbotToken = config.RegisterOption("yagpdb.owlbot_token", "Owlbot API token", "")

func ShouldRegister() bool {
	return confOwlbotToken.GetString() != ""
}

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "OwlDictionary",
	Aliases:      []string{"owldict", "owl"},
	Description:  "Get the definition of an English word using the Owlbot API.",
	RequiredArgs: 1,
	Cooldown:     5,
	Arguments: []*dcmd.ArgDef{
		{Name: "Query", Help: "Word to search for", Type: dcmd.String},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		query := strings.ToLower(data.Args[0].Str())
		req, err := http.NewRequest("GET", "https://owlbot.info/api/v4/dictionary/"+url.QueryEscape(query), nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Token "+confOwlbotToken.GetString())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return "Could not find a definition for that word.", nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var res OwlbotResult
		err = json.Unmarshal(body, &res)
		if err != nil || len(res.Definitions) == 0 {
			return "Could not find a definition for that word.", err
		}

		if len(res.Definitions) == 1 || data.Context().Value(paginatedmessages.CtxKeyNoPagination) != nil {
			return createOwlbotDefinitionEmbed(&res, res.Definitions[0]), nil
		}

		_, err = paginatedmessages.CreatePaginatedMessage(data.GuildData.GS.ID, data.ChannelID, 1, len(res.Definitions), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			if page > len(res.Definitions) {
				return nil, paginatedmessages.ErrNoResults
			}

			return createOwlbotDefinitionEmbed(&res, res.Definitions[page-1]), nil
		})

		return nil, err
	},
}

func createOwlbotDefinitionEmbed(res *OwlbotResult, def *OwlbotDefinition) *discordgo.MessageEmbed {
	title := strings.Title(normalizeOutput(res.Word))
	if def.Emoji != nil {
		title = title + " " + normalizeOutput(*def.Emoji)
	}

	embed := &discordgo.MessageEmbed{
		Author:      &discordgo.MessageEmbedAuthor{Name: "Owlbot", IconURL: "https://i.imgur.com/zgBXENZ.png", URL: "https://owlbot.info/"},
		Title:       title,
		Description: common.CutStringShort(capitalizeSentences(normalizeOutput(def.Definition)), 2048),
		Color:       0x07AB99,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if def.ImageURL != nil {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: *def.ImageURL}
	}

	if res.Pronunciation != nil {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Pronunciation",
			Value:  normalizeOutput(*res.Pronunciation),
			Inline: true,
		})
	}

	if def.Type != nil {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Type",
			Value:  strings.Title(normalizeOutput(*def.Type)),
			Inline: true,
		})
	}

	if def.Example != nil {
		example := capitalizeSentences(normalizeOutput(*def.Example))
		if !hasEndOfSentenceSymbol(example) {
			example = example + "." // add period if no other symbol that ends the sentence is present
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Example",
			Value:  "*" + common.CutStringShort(example, 1022) + "*",
			Inline: false,
		})
	}

	return embed
}

var policy = bluemonday.StrictPolicy()

func normalizeOutput(s string) string {
	// The API occasionally returns HTML tags and escapes as part of output, remove them.
	decoded := html.UnescapeString(policy.Sanitize(s))
	// It also sometimes returns non-printable characters, strip them out too.
	return strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, decoded)
}

func capitalizeSentences(s string) string {
	var builder strings.Builder

	capitalizeCur := true // whether the current phrase should be capitalized.
	for i, word := range strings.Fields(s) {
		if i > 0 {
			builder.WriteByte(' ')
		}

		if capitalizeCur {
			// strings.Title() does not work properly with punctuation: for example, "what's" becomes 'What'S" when passed to it, which is undesirable.
			// Instead, title-case the first rune manually and write the rest as is, as we know `word` represents a single word.
			r, size := utf8.DecodeRuneInString(word)
			if r == utf8.RuneError {
				// fall back to original text
				builder.WriteString(word)
			} else {
				builder.WriteRune(unicode.ToTitle(r))
				builder.WriteString(word[size:])
			}
		} else {
			builder.WriteString(word)
		}

		capitalizeCur = hasEndOfSentenceSymbol(word)
	}

	return builder.String()
}

func hasEndOfSentenceSymbol(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[len(s)-1] {
	case '.', '?', '!':
		return true
	default:
		return false
	}
}

type OwlbotResult struct {
	Word          string              `json:"word"`
	Definitions   []*OwlbotDefinition `json:"definitions"`
	Pronunciation *string             `json:"pronunciation"`
}

type OwlbotDefinition struct {
	Type       *string `json:"type"`
	Definition string  `json:"definition"`
	Example    *string `json:"example"`
	ImageURL   *string `json:"image_url"`
	Emoji      *string `json:"emoji"`
}
