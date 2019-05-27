package weather

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
	"github.com/lunixbochs/vtclean"
)

var TempRangeRegex = regexp.MustCompile("(-?[0-9]{1,3})( ?- ?(-?[0-9]{1,3}))? ?°C")

var Command = &commands.YAGCommand{
	CmdCategory:  commands.CategoryFun,
	Name:         "Weather",
	Aliases:      []string{"w"},
	Description:  "Shows the weather somewhere",
	RunInDM:      true,
	RequiredArgs: 1,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Where", Type: dcmd.String},
	},
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

			tmpFrom := 0
			tmpTo := 0
			isRange := false

			submatches := TempRangeRegex.FindStringSubmatch(v)
			if len(submatches) < 2 {
				continue
			}

			tmpFrom, _ = strconv.Atoi(submatches[1])

			if len(submatches) >= 4 && submatches[3] != "" {
				tmpTo, _ = strconv.Atoi(submatches[3])
				isRange = true
			}

			// convert to fahernheit
			tmpFrom = int(float64(tmpFrom)*1.8 + 32)
			tmpTo = int(float64(tmpTo)*1.8 + 32)

			v = strings.TrimRight(v, " ")
			if isRange {
				split[i] = v + fmt.Sprintf(" (%d-%d °F)", tmpFrom, tmpTo)
			} else {
				split[i] = v + fmt.Sprintf(" (%d °F)", tmpFrom)
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
