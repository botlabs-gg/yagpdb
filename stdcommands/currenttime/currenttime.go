package currenttime

import (
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/timezonecompanion"
	"github.com/tkuchiki/go-timezone"
)

var Command = &commands.YAGCommand{
	CmdCategory:    commands.CategoryTool,
	Name:           "CurrentTime",
	Aliases:        []string{"ctime", "gettime"},
	Description:    "Shows current time in different timezones. [Available timezones](https://pastebin.com/ZqSPUhc7)",
	ArgumentCombos: [][]int{{1}, {0}, {}},
	Arguments: []*dcmd.ArgDef{
		{Name: "Zone", Type: dcmd.String},
		{Name: "Offset", Type: dcmd.Int},
	},
	RunFunc: cmdFuncCurrentTime,
}

func cmdFuncCurrentTime(data *dcmd.Data) (interface{}, error) {
	const format = "Mon Jan 02 15:04:05 (UTC -07:00)"

	now := time.Now()
	if data.Args[0].Value != nil {
		tzName := data.Args[0].Str()
		names, err := timezone.GetTimezones(strings.ToUpper(data.Args[0].Str()))
		if err == nil && len(names) > 0 {
			tzName = names[0]
		}

		location, err := time.LoadLocation(tzName)
		if err != nil {
			if offset, ok := customTZOffsets[strings.ToUpper(tzName)]; ok {
				location = time.FixedZone(tzName, int(offset*60*60))
			} else {
				return "Unknown timezone :(", err
			}
		}
		return now.In(location).Format(format), nil
	} else if data.Args[1].Value != nil {
		location := time.FixedZone("", data.Args[1].Int()*60*60)
		return now.In(location).Format(format), nil
	}

	loc := timezonecompanion.GetUserTimezone(data.Author.ID)
	if loc != nil {
		return now.In(loc).Format(format), nil
	}

	return now.Format(format), nil
}
