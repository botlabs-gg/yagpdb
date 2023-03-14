package customcommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestCheckMatch(t *testing.T) {
	tests := []struct {
		// Have
		cmd *models.CustomCommand
		msg string
		// Want
		match bool
		args  []string
	}{
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerCommand),
				TextTrigger: "freezeit",
			},
			"!!!freezeit then cut\\ it",
			true,
			[]string{"!!!freezeit", "then", "cut it"},
		},
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerCommand),
				TextTrigger: "freezeit",
			},
			"freezeit then cut\\ it",
			false,
			nil,
		},
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerStartsWith),
				TextTrigger: "freezeit",
			},
			"freezeit then cut\\ it",
			true,
			[]string{"freezeit", "then", "cut it"},
		},
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerContains),
				TextTrigger: "freezeit",
			},
			"I want you to freezeit then cut\\ it",
			true,
			[]string{"I want you to freezeit", "then", "cut it"},
		},
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerRegex),
				TextTrigger: "f.*?it",
			},
			"I want you to freezeit then cut\\ it",
			true,
			[]string{"I want you to freezeit", "then", "cut it"},
		},
		{
			&models.CustomCommand{
				TriggerType: int(CommandTriggerRegex),
				TextTrigger: "freezeit then cut it",
			},
			"freezeit then cut it",
			true,
			[]string{"freezeit then cut it"},
		},
	}

	common.BotUser = &discordgo.User{}

	for i, test := range tests {
		m, _, a := CheckMatch("!!!", test.cmd, test.msg)
		if m != test.match {
			t.Errorf("%d: got match '%t', want match '%t'", i, m, test.match)
		}
		if a == nil && test.args != nil {
			t.Errorf("%d: got no args, wanted args", i)
		} else if a != nil && test.args == nil {
			t.Errorf("%d: got args '%q', wanted no args", i, a)
		} else if len(a) != len(test.args) {
			t.Errorf("%d: got args '%q', wanted args '%q'", i, a, test.args)
		} else {
			for j, v := range test.args {
				if len(a) < j || a[j] != v {
					t.Errorf("%d: got arg %d %q, wanted arg %q", i, j, a[j], v)
				}
			}
		}
	}
}
