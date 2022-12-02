package topevents

import (
	"fmt"
	"sort"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	Cooldown:     2,
	CmdCategory:  commands.CategoryDebug,
	Name:         "topevents",
	Description:  "Shows gateway event processing stats for all or one shard",
	HideFromHelp: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "shard", Type: dcmd.Int},
	},
	RunFunc: cmdFuncTopEvents,
}

func cmdFuncTopEvents(data *dcmd.Data) (interface{}, error) {

	shardsTotal, lastPeriod := bot.EventLogger.GetStats()

	sortable := make([]*DiscordEvtEntry, len(eventsystem.AllDiscordEvents))
	for i, _ := range sortable {
		sortable[i] = &DiscordEvtEntry{
			Name: eventsystem.AllDiscordEvents[i].String(),
		}
	}

	for i, _ := range shardsTotal {
		if data.Args[0].Value != nil && data.Args[0].Int() != i {
			continue
		}

		for de, j := range eventsystem.AllDiscordEvents {
			sortable[de].Total += shardsTotal[i][j]
			sortable[de].PerSecond += float64(lastPeriod[i][j]) / bot.EventLoggerPeriodDuration.Seconds()
		}
	}

	sort.Sort(DiscordEvtEntrySortable(sortable))

	out := "Total event stats across all shards:\n"
	if data.Args[0].Value != nil {
		out = fmt.Sprintf("Stats for shard %d:\n", data.Args[0].Int())
	}

	out += "\n#     Total  -   /s  - Event\n"
	sum := int64(0)
	sumPerSecond := float64(0)
	for k, entry := range sortable {
		out += fmt.Sprintf("#%-2d: %7d - %5.1f - %s\n", k+1, entry.Total, entry.PerSecond, entry.Name)
		sum += entry.Total
		sumPerSecond += entry.PerSecond
	}

	out += fmt.Sprintf("\nTotal: %d, Events per second: %.1f", sum, sumPerSecond)
	out += "\n"

	return out, nil
}

type DiscordEvtEntry struct {
	Name      string
	Total     int64
	PerSecond float64
}

type DiscordEvtEntrySortable []*DiscordEvtEntry

func (d DiscordEvtEntrySortable) Len() int {
	return len(d)
}

func (d DiscordEvtEntrySortable) Less(i, j int) bool {
	return d[i].Total > d[j].Total
}

func (d DiscordEvtEntrySortable) Swap(i, j int) {
	temp := d[i]
	d[i] = d[j]
	d[j] = temp
}
