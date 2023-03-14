package weather

import (
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/lunixbochs/vtclean"
)

var (
	TempRangeRegex1 = regexp.MustCompile("(-?[0-9]{1,3})( ?- ?(-?[0-9]{1,3}))? ?°C")
	TempRangeRegex2 = regexp.MustCompile(`(-?\+?[0-9]{1,3})( ?\((-?[0-9]{1,3})\))? ?°C`)
	NumberRegex     = regexp.MustCompile(`-?\d{1,3}`)
)

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Weather",
	Aliases:      []string{"w"},
	Description:  "Shows the weather somewhere",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		{Name: "Where", Type: dcmd.String},
	},
	DefaultEnabled:      true,
	SlashCommandEnabled: true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		where := data.Args[0].Str()

		req, err := http.NewRequest("GET", "http://wttr.in/"+where+"?m", nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", "curl/7.49.1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// remove escape sequences
		unescaped := vtclean.Clean(string(body), false)

		split := strings.Split(string(unescaped), "\n")

		// Show both celcius and fahernheit
		for i, v := range split {
			if !strings.Contains(v, "°C") {
				continue
			}

			if newS, converted := convTempFormat(TempRangeRegex1, v); converted {
				split[i] = newS
			} else if newS, converted := convTempFormat(TempRangeRegex2, v); converted {
				split[i] = newS
			}
		}

		out := "```\n"
		for i := 0; i < 7; i++ {
			if i >= len(split) {
				break
			}
			out += strings.TrimRight(split[i], " ") + "\n"
		}
		out += "\n```"

		return out, nil
	},
}

func convTempFormat(regex *regexp.Regexp, input string) (string, bool) {
	pos := regex.FindStringIndex(input)
	if pos == nil {
		return input, false
	}

	rest := strings.TrimSpace(input[pos[0]:])

	doneso := NumberRegex.ReplaceAllStringFunc(rest, func(s string) string {
		celcius, err := strconv.Atoi(s)
		if err != nil {
			return s
		}

		converted := int(float64(celcius)*1.8 + 32)
		return strconv.Itoa(converted)
	})

	doneso = strings.ReplaceAll(doneso, "°C", "°F")

	return strings.TrimRightFunc(input, unicode.IsSpace) + " / " + doneso, true
}
