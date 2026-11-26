package customcommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func TestMatchMessageTriggersEditLimit(t *testing.T) {
	var cmds []*models.CustomCommand
	for i := 1; i <= 5; i++ {
		cmds = append(cmds, &models.CustomCommand{
			LocalID:       int64(i),
			TriggerType:   int(CommandTriggerContains),
			TextTrigger:   "hello",
			TriggerOnEdit: i >= 4,
			R:             (&models.CustomCommand{}).R.NewStruct(),
		})
	}

	cs := &dstate.ChannelState{ID: 1, GuildID: 2}
	ms := &dstate.MemberState{Member: &dstate.MemberFields{}}
	msg := &discordgo.Message{Content: "hello there", GuildID: 2}

	ids := func(matched []*TriggeredCC) []int64 {
		var out []int64
		for _, m := range matched {
			out = append(out, m.CC.LocalID)
		}
		return out
	}

	created := ids(matchMessageTriggers(cmds, "-", cs, ms, msg, false, 3))
	if len(created) != 3 || created[0] != 1 || created[2] != 3 {
		t.Errorf("new message matched %v, want [1 2 3]", created)
	}

	edited := ids(matchMessageTriggers(cmds, "-", cs, ms, msg, true, 3))
	if len(edited) != 2 || edited[0] != 4 || edited[1] != 5 {
		t.Errorf("edit matched %v, want [4 5]", edited)
	}
}
