package forex

import (
	"encoding/json"
	"fmt"
	"io"

	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryFun,
	Cooldown:            5,
	Name:                "Forex",
	Aliases:             []string{"Money"},
	Description:         "💱 convert value from one currency to another.",
	RunInDM:             true,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RequiredArgs:        3,
	Arguments: []*dcmd.ArgDef{
		{Name: "Amount", Type: dcmd.Float}, {Name: "From", Type: dcmd.String}, {Name: "To", Type: dcmd.String},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		amount := data.Args[0].Float64()
		check, err := requestAPI("https://api.exchangerate.host/symbols")
		if err != nil {
			return nil, err
		}
		from := check.Symbols[strings.ToUpper(data.Args[1].Str())]
		to := check.Symbols[strings.ToUpper(data.Args[2].Str())]
		// Checks the max amount of pages by the number of symbols on each page (15)
		maxPages := int(math.Ceil(float64(len(check.Symbols)) / float64(15)))
		if (to == nil) || (from == nil) {
			_, err = paginatedmessages.CreatePaginatedMessage(
				data.GuildData.GS.ID, data.ChannelID, 1, maxPages, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					return errEmbed(check, page)
				})
			if err != nil {
				return nil, err
			}
			return nil, nil
		}
		output, err := requestAPI(fmt.Sprintf("https://api.exchangerate.host/convert?from=%s&to=%s&amount=%.3f", from.Code, to.Code, amount))
		if err != nil {
			return nil, err
		}
		p := message.NewPrinter(language.English)
		embed := &discordgo.MessageEmbed{
			Title:       "💱Currency Exchange Rate",
			Description: p.Sprintf("\n%.2f **%s** (%s) is %.3f **%s** (%s).", amount, from.Description, output.Query.From, output.Result, to.Description, output.Query.To),
			Color:       0xAE27FF,
			Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Based on currency rate 1 : %f", output.Info.Rate)},
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}
		return embed, nil
	},
}

func requestAPI(query string) (*ExchangeAPIResult, error) {
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, commands.NewPublicError("HTTP err: ", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &ExchangeAPIResult{}
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func errEmbed(check *ExchangeAPIResult, page int) (*discordgo.MessageEmbed, error) {
	desc := "CODE | Description\n------------------"
	var exchangeSymbols string = "https://api.exchangerate.host/symbols"
	codes := make([]string, 0, len(check.Symbols))
	for k := range check.Symbols {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	start := (page * 15) - 15
	end := page * 15
	for i, c := range codes {
		if i < end && i >= start {
			desc = fmt.Sprintf("%s\n%s  | %s", desc, c, check.Symbols[c].Description)
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Invalid currency code",
		URL:         exchangeSymbols,
		Color:       0xAE27FF,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Description: fmt.Sprintf("Check out available codes on: %s ```\n%s```", exchangeSymbols, desc),
	}
	return embed, nil
}

type CurrencySymbolInfo struct {
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
}

type ExchangeAPIResult struct {
	Motd *struct {
		Msg string `json:"msg"`
		URL string `json:"url"`
	} `json:"motd"`
	Success bool `json:"success"`
	Query   *struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Amount float64 `json:"amount"`
	} `json:"query,omitempty"`
	Info *struct {
		Rate float64 `json:"rate"`
	} `json:"info,omitempty"`
	Historical bool                           `json:"historical,omitempty"`
	Date       string                         `json:"date,omitempty"`
	Result     float64                        `json:"result,omitempty"`
	Symbols    map[string]*CurrencySymbolInfo `json:"symbols,omitempty"`
}
