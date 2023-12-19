package forex

import (
	"encoding/json"
	"fmt"

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

const currenciesAPIURL = "https://api.frankfurter.app/currencies"
const currencyPerPage = 16

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
		var currenciesResult Currencies
		var exchangeRateResult ExchangeRate

		err := requestAPI(currenciesAPIURL, &currenciesResult)
		if err != nil {
			return nil, err
		}

		from := strings.ToUpper(data.Args[1].Str())
		to := strings.ToUpper(data.Args[2].Str())

		// Check if the currencies exist in the map
		_, fromExist := currenciesResult[from]
		_, toExist := currenciesResult[to]

		// Checks the max amount of pages by the number of symbols on each page
		maxPages := int(math.Ceil(float64(len(currenciesResult)) / float64(currencyPerPage)))

		// If the currency isn't supported by API.
		if !toExist || !fromExist {
			_, err = paginatedmessages.CreatePaginatedMessage(
				data.GuildData.GS.ID, data.ChannelID, 1, maxPages, func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
					embed, err := errEmbed(currenciesResult, page)
					if err != nil {
						return nil, err
					}
					return embed, nil
				})
			if err != nil {
				return nil, err
			}
			return nil, nil
		}

		err = requestAPI(fmt.Sprintf("https://api.frankfurter.app/latest?amount=%.3f&from=%s&to=%s", amount, from, to), &exchangeRateResult)
		if err != nil {
			return nil, err
		}

		p := message.NewPrinter(language.English)
		embed := &discordgo.MessageEmbed{
			Title:       "💱Currency Exchange Rate",
			Description: p.Sprintf("\n%.2f **%s** (%s) is %.3f **%s** (%s).", amount, currenciesResult[from], from, exchangeRateResult.Rates[to], currenciesResult[to], to),
			Color:       0xAE27FF,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}
		return embed, nil
	},
}

func requestAPI(query string, result interface{}) error {
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return commands.NewPublicError("Failed to convert, Please verify your input")
	}

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return err
	}
	return nil
}

func errEmbed(currenciesResult Currencies, page int) (*discordgo.MessageEmbed, error) {
	desc := "CODE | Description\n------------------"
	codes := make([]string, 0, len(currenciesResult))
	for k := range currenciesResult {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	start := (page * currencyPerPage) - currencyPerPage
	end := page * currencyPerPage
	for i, c := range codes {
		if i < end && i >= start {
			desc = fmt.Sprintf("%s\n%s  | %s", desc, c, currenciesResult[c])
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Invalid currency code",
		URL:         currenciesAPIURL,
		Color:       0xAE27FF,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Description: fmt.Sprintf("Check out available codes on: %s ```\n%s```", currenciesAPIURL, desc),
	}
	return embed, nil
}

type ExchangeRate struct {
	Amount float64
	Base   string
	Date   string
	Rates  map[string]float64
}
type Currencies map[string]string
