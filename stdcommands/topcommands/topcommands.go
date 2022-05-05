package topcommands

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
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
	RunFunc: cmdFuncTopCommands,
}

func cmdFuncTopCommands(data *dcmd.Data) (interface{}, error) {
	hours := data.Args[0].Int()
	within := time.Now().Add(time.Duration(-hours) * time.Hour)

	var results []*TopCommandsResult
	err := common.GORM.Table(common.LoggedExecutedCommand{}.TableName()).Select("command, COUNT(id)").Where("created_at > ?", within).Group("command").Order("count(id) desc").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	out := fmt.Sprintf("```\nCommand stats from now to %d hour(s) ago\n#    Total -  Command\n", hours)
	total := 0
	for k, result := range results {
		out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Command)
		total += result.Count
	}

	cpm := float64(total) / float64(hours) / 60

	out += fmt.Sprintf("\nTotal: %d, Commands per minute: %.1f", total, cpm)
	out += "\n```"

	return out, nil
}

type TopCommandsResult struct {
	Command string
	Count   int
}
