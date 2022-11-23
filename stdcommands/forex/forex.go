package forex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	Description:         "💱 convert value from one currency for another.",
	RunInDM:             true,
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RequiredArgs:        3,
	Arguments: []*dcmd.ArgDef{
		{Name: "Amount", Type: dcmd.Int}, {Name: "From", Type: dcmd.String}, {Name: "To", Type: dcmd.String},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		amount := data.Args[0]
		check, err := requestAPI("https://api.exchangerate.host/symbols")
		if err != nil {
			return nil, err
		}
		from := check.Symbols[strings.ToUpper(data.Args[1].Str())]
		to := check.Symbols[strings.ToUpper(data.Args[2].Str())]
		if (to == nil) || (from == nil) {
			return "Invalid currency code.\nCheck out available codes on: <https://api.exchangerate.host/symbols>", nil
		}
		output, err := requestAPI("https://api.exchangerate.host/convert?from=" + from.Code + "&to=" + to.Code + "&amount=" + amount.Str())
		if err != nil {
			return nil, err
		}
		if output.Info.Rate == 0 {
			return "Something went wrong :c", err //VEF is bugged but i dont see any way to fix it yet. other than API fixing it themself :/
		}
		p := message.NewPrinter(language.English)
		embed := &discordgo.MessageEmbed{
			Title:       "💱Currency Exchange Rate",
			Description: fmt.Sprintf("\n%s **%s** (%s) is %s **%s** (%s).", p.Sprintf("%d", amount.Int64()), from.Description, output.Query.From, p.Sprintf("%0.2f", output.Result), to.Description, output.Query.To),
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
	req.Header.Set("User-Agent", "curl/7.83.1")
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
		From   string `json:"from"`
		To     string `json:"to"`
		Amount int    `json:"amount"`
	} `json:"query,omitempty"`
	Info *struct {
		Rate float64 `json:"rate"`
	} `json:"info,omitempty"`
	Historical bool                           `json:"historical,omitempty"`
	Date       string                         `json:"date,omitempty"`
	Result     float64                        `json:"result,omitempty"`
	Symbols    map[string]*CurrencySymbolInfo `json:"symbols,omitempty"`
}