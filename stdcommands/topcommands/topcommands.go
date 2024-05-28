package topcommands

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:     2,
	CmdCategory:  commands.CategoryDebug,
	Name:         "topcommands",
	Description:  "Shows command usage stats",
	HideFromHelp: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "hours", Type: dcmd.Int, Default: 1},
	},
	RunFunc: util.RequireBotAdmin(cmdFuncTopCommands),
}

func cmdFuncTopCommands(data *dcmd.Data) (interface{}, error) {
	hours := data.Args[0].Int()
	within := time.Now().Add(time.Duration(-hours) * time.Hour)

	const q = `
SELECT command, COUNT(id)
FROM executed_commands
WHERE created_at > $1
GROUP BY command
ORDER BY COUNT(id) DESC;
`
	rows, err := common.PQ.Query(q, within)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := fmt.Sprintf("```\nCommand stats from now to %d hour(s) ago\n#    Total -  Command\n", hours)
	total := 0
	for k := 1; rows.Next(); k++ {
		var command string
		var count int
		err = rows.Scan(&command, &count)
		if err != nil {
			return nil, err
		}

		out += fmt.Sprintf("#%02d: %5d - %s\n", k, count, command)
		total += count
	}

	cpm := float64(total) / float64(hours) / 60

	out += fmt.Sprintf("\nTotal: %d, Commands per minute: %.1f", total, cpm)
	out += "\n```"

	return out, nil
}
