package customcommands

import (
	"testing"
)

func TestCheckMatch(t *testing.T) {
	tests := []struct {
		// Have
		cmd CustomCommand
		msg string
		// Want
		match bool
		args  []string
	}{
		{
			CustomCommand{
				TriggerType: CommandTriggerCommand,
				Trigger:     "freezeit",
			},
			"!!!freezeit then cut\\ it",
			true,
			[]string{"!!!freezeit", "then", "cut it"},
		},
		{
			CustomCommand{
				TriggerType: CommandTriggerCommand,
				Trigger:     "freezeit",
			},
			"freezeit then cut\\ it",
			false,
			nil,
		},
		{
			CustomCommand{
				TriggerType: CommandTriggerStartsWith,
				Trigger:     "freezeit",
			},
			"freezeit then cut\\ it",
			true,
			[]string{"freezeit", "then", "cut it"},
		},
		{
			CustomCommand{
				TriggerType: CommandTriggerContains,
				Trigger:     "freezeit",
			},
			"I want you to freezeit then cut\\ it",
			true,
			[]string{"I want you to freezeit", "then", "cut it"},
		},
		{
			CustomCommand{
				TriggerType: CommandTriggerRegex,
				Trigger:     "f.*?it",
			},
			"I want you to freezeit then cut\\ it",
			true,
			[]string{"I want you to freezeit", "then", "cut it"},
		},
		{
			CustomCommand{
				TriggerType: CommandTriggerRegex,
				Trigger:     "freezeit then cut it",
			},
			"freezeit then cut it",
			true,
			[]string{"freezeit then cut it"},
		},
	}

	for i, test := range tests {
		m, a := CheckMatch("!!!", &test.cmd, test.msg)
		if m != test.match {
			t.Errorf("%d: got match '%b', want match '%b'", i, m, test.match)
		}
		if a == nil && test.args != nil {
			t.Errorf("%d: got no args, wanted args", i)
		} else if a != nil && test.args == nil {
			t.Errorf("%d: got args '%q', wanted no args", i, a)
		} else if len(a) != len(test.args) {
			t.Errorf("%d: got args '%q', wanted args '%q'", i, a, test.args)
		} else {
			for i, v := range test.args {
				if len(a) < i || a[i] != v {
					t.Errorf("%d: got arg %d '%q', wanted arg '%q'", i, a[i], v)
				}
			}
		}
	}
}
