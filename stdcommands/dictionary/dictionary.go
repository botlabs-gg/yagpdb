package dictionary

import (
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sirupsen/logrus"
)

const (
	apiEndpoint = "https://freedictionaryapi.com/api/v1/entries/en/"

	// The data is Wiktionary content under CC BY-SA 4.0, which obliges us to
	// credit the source and link the original page.
	attributionText = "Powered by freedictionaryapi.com"
	attributionIcon = "https://upload.wikimedia.org/wikipedia/commons/thumb/e/ec/Wiktionary-logo.svg/250px-Wiktionary-logo.svg.png"

	maxDescriptionLength = 2048
	maxFieldLength       = 1024
	maxListedTerms       = 10

	// Longer than any real word, and it keeps junk input out of the cache keys
	// and off the api.
	maxQueryLength = 64

	// The api allows 1000 requests an hour per ip and exposes no budget headers,
	// so repeat lookups are served from redis instead. Definitions are static
	// enough for a long ttl; misses expire sooner in case wiktionary gains the
	// word.
	cacheKeyPrefix  = "dictionary:en:"
	cacheTTLFound   = 24 * 60 * 60
	cacheTTLMissing = 60 * 60
)

// The upstream dictionary can stop answering without closing the connection, so
// a request must not be allowed to pin a goroutine indefinitely.
var httpClient = &http.Client{Timeout: 10 * time.Second}

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "dictionary",
	Aliases:      []string{"owldict", "owl", "dict"},
	Description:  "Get the definition of an English word using freedictionaryapi.com",
	RequiredArgs: 1,
	Cooldown:     5,
	Arguments: []*dcmd.ArgDef{
		{Name: "Query", Help: "Word to search for", Type: dcmd.String},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		query := strings.ToLower(data.Args[0].Str())

		res, err := lookup(query)
		if errors.Is(err, errRateLimited) {
			return "The dictionary is being used too heavily right now, try again in a bit.", nil
		}
		if err != nil {
			return nil, err
		}

		if res == nil || len(res.Entries) == 0 {
			return "Could not find a definition for that word.", nil
		}

		entries := res.Entries
		if len(entries) == 1 || data.Context().Value(paginatedmessages.CtxKeyNoPagination) != nil {
			return createDictionaryDefinitionEmbed(res, &entries[0]), nil
		}

		return paginatedmessages.NewPaginatedResponse(data.GuildData.GS.ID, data.ChannelID, 1, len(entries), func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			if page > len(entries) {
				return nil, paginatedmessages.ErrNoResults
			}

			return createDictionaryDefinitionEmbed(res, &entries[page-1]), nil
		}), nil
	},
}

// A breach of the hourly quota is only visible as a 429, so it is reported to
// the user rather than logged as a command failure.
var errRateLimited = errors.New("dictionary api rate limited")

func lookup(query string) (*DictionaryResponse, error) {
	if query == "" || utf8.RuneCountInString(query) > maxQueryLength {
		return &DictionaryResponse{Word: query}, nil
	}

	cacheKey := cacheKeyPrefix + query

	var cached DictionaryResponse
	if err := common.GetCacheDataJson(cacheKey, &cached); err == nil {
		return &cached, nil
	}

	res, err := fetch(query)
	if err != nil {
		return nil, err
	}

	ttl := cacheTTLFound
	if len(res.Entries) == 0 {
		ttl = cacheTTLMissing
	}
	if err := common.SetCacheDataJson(cacheKey, ttl, res); err != nil {
		// A cold cache only costs an extra request, so this must not fail the command.
		logrus.WithError(err).Warn("Failed caching dictionary lookup")
	}

	return res, nil
}

func fetch(query string) (*DictionaryResponse, error) {
	req, err := http.NewRequest("GET", apiEndpoint+url.PathEscape(query), nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		// The api answers unknown words with 200 and no entries, this is only a
		// guard against that changing.
		return &DictionaryResponse{Word: query}, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, errRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("dictionary api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res DictionaryResponse
	if err := json.Unmarshal(body, &res); err != nil {
		logrus.WithError(err).Error("Failed decoding response from freedictionaryapi")
		return nil, err
	}

	return &res, nil
}

func createDictionaryDefinitionEmbed(res *DictionaryResponse, entry *Entry) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       capitalize(normalizeOutput(res.Word)),
		Description: definitionList(entry),
		Color:       0x07AB99,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer:      attributionFooter(res),
	}

	if res.Source.URL != "" {
		embed.URL = res.Source.URL
	}

	if pronunciation := pronunciationList(entry); pronunciation != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Pronunciation",
			Value:  pronunciation,
			Inline: true,
		})
	}

	if entry.PartOfSpeech != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Type",
			Value:  capitalize(normalizeOutput(entry.PartOfSpeech)),
			Inline: true,
		})
	}

	// Senses carry their own synonyms, so fall back to those when the entry
	// itself lists none.
	synonyms, antonyms := entry.Synonyms, entry.Antonyms
	for _, s := range entry.Senses {
		if len(synonyms) == 0 {
			synonyms = s.Synonyms
		}
		if len(antonyms) == 0 {
			antonyms = s.Antonyms
		}
	}

	if field := termList(synonyms); field != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Synonyms", Value: field, Inline: true})
	}
	if field := termList(antonyms); field != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Antonyms", Value: field, Inline: true})
	}

	return embed
}

// attributionFooter credits the api and names the licence the response reports,
// which CC BY-SA 4.0 requires alongside the source link carried in the embed url.
func attributionFooter(res *DictionaryResponse) *discordgo.MessageEmbedFooter {
	text := attributionText
	if license := normalizeOutput(res.Source.License.Name); license != "" {
		text += " | Wiktionary, " + license
	}

	return &discordgo.MessageEmbedFooter{
		Text:    text,
		IconURL: attributionIcon,
	}
}

func definitionList(entry *Entry) string {
	var b strings.Builder

	for _, sense := range entry.Senses {
		definition := normalizeOutput(sense.Definition)
		if definition == "" {
			continue
		}

		line := "\n- "
		if tags := formatTags(sense.Tags, definition); tags != "" {
			line += tags + " "
		}
		line += definition

		var example string
		if len(sense.Examples) > 0 {
			example = normalizeOutput(sense.Examples[0])
			if example != "" {
				if !hasEndOfSentenceSymbol(example) {
					example += "."
				}
				example = fmt.Sprintf("\n**Example:** *%s*", example)
			}
		}

		if b.Len()+len(line)+len(example) > maxDescriptionLength {
			break
		}

		b.WriteString(line)
		b.WriteString(example)
	}

	return common.CutStringShort(strings.TrimLeft(b.String(), "\n"), maxDescriptionLength)
}

// formatTags renders the usage labels wiktionary attaches to a sense, such as
// archaic or slang, which the previous api did not provide at all.
//
// The definition text frequently opens with the same labels as a parenthetical,
// so rendering both would read "(transitive) (transitive) To put ...".
func formatTags(tags []string, definition string) string {
	if strings.HasPrefix(definition, "(") {
		return ""
	}

	cleaned := make([]string, 0, len(tags))
	for _, t := range tags {
		if t = normalizeOutput(t); t != "" {
			cleaned = append(cleaned, t)
		}
	}

	if len(cleaned) == 0 {
		return ""
	}

	return "*(" + strings.Join(cleaned, ", ") + ")*"
}

func pronunciationList(entry *Entry) string {
	var lines []string
	seen := make(map[string]bool)

	for _, p := range entry.Pronunciations {
		text := normalizeOutput(p.Text)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true

		line := text
		if accent := strings.Join(p.Tags, ", "); accent != "" {
			line += " *(" + normalizeOutput(accent) + ")*"
		}
		lines = append(lines, line)
	}

	return common.CutStringShort(strings.Join(lines, "\n"), maxFieldLength)
}

func termList(terms []string) string {
	cleaned := make([]string, 0, len(terms))
	for _, t := range terms {
		if t = normalizeOutput(t); t != "" {
			cleaned = append(cleaned, t)
		}
		if len(cleaned) >= maxListedTerms {
			break
		}
	}

	return common.CutStringShort(strings.Join(cleaned, ", "), maxFieldLength)
}

var policy = bluemonday.StrictPolicy()

func normalizeOutput(s string) string {
	// Defensive: the source is user edited wiktionary content.
	decoded := html.UnescapeString(policy.Sanitize(s))
	// Strip non-printable characters that occasionally slip through.
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, decoded))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}

	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}

	return string(unicode.ToTitle(r)) + s[size:]
}

func hasEndOfSentenceSymbol(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[len(s)-1] {
	case '.', '?', '!', ':', '"':
		return true
	default:
		return false
	}
}

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Pronunciation struct {
	Type string   `json:"type"`
	Text string   `json:"text"`
	Tags []string `json:"tags"`
}

type Form struct {
	Word string   `json:"word"`
	Tags []string `json:"tags"`
}

type Sense struct {
	Definition string   `json:"definition"`
	Tags       []string `json:"tags"`
	Examples   []string `json:"examples"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
}

type Entry struct {
	Language       Language        `json:"language"`
	PartOfSpeech   string          `json:"partOfSpeech"`
	Pronunciations []Pronunciation `json:"pronunciations"`
	Forms          []Form          `json:"forms"`
	Senses         []Sense         `json:"senses"`
	Synonyms       []string        `json:"synonyms"`
	Antonyms       []string        `json:"antonyms"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Source struct {
	URL     string  `json:"url"`
	License License `json:"license"`
}

type DictionaryResponse struct {
	Word    string  `json:"word"`
	Entries []Entry `json:"entries"`
	Source  Source  `json:"source"`
}
