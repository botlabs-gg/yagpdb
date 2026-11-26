package customcommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestReactionMemberWithoutFetch(t *testing.T) {
	if ms := reactionMemberWithoutFetch(1, 2, nil); ms != nil {
		t.Errorf("expected nil without state or event member, got %+v", ms)
	}

	member := &discordgo.Member{User: &discordgo.User{ID: 2}, Roles: []int64{3}}
	ms := reactionMemberWithoutFetch(1, 2, member)
	if ms == nil || ms.GuildID != 1 || ms.User.ID != 2 || len(ms.Member.Roles) != 1 {
		t.Fatalf("expected member from the event, got %+v", ms)
	}
	if member.GuildID != 0 {
		t.Error("the event member should not be modified")
	}
}
