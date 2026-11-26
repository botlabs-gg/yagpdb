package customcommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func TestDelayedRunFrameDropsMentions(t *testing.T) {
	live := &templates.ContextFrame{
		MentionEveryone: true,
		MentionHere:     true,
		MentionRoles:    []int64{1, 2},
		DelResponse:     true,
	}

	persisted := delayedRunFrame(live)
	if persisted.MentionEveryone || persisted.MentionHere || len(persisted.MentionRoles) != 0 {
		t.Fatalf("persisted frame kept mention permissions: %+v", persisted)
	}
	if !persisted.DelResponse {
		t.Fatalf("persisted frame lost unrelated fields")
	}
	if !live.MentionEveryone || !live.MentionHere || len(live.MentionRoles) != 2 {
		t.Fatalf("live frame was mutated: %+v", live)
	}
	if delayedRunFrame(nil) != nil {
		t.Fatalf("expected nil frame to stay nil")
	}
}

func TestRoleTriggerMessageAuthoredByTarget(t *testing.T) {
	target := &dstate.MemberState{
		User:    discordgo.User{ID: 42},
		GuildID: 1,
		Member:  &dstate.MemberFields{},
	}

	msg := roleTriggerMessage(1, 2, target)
	if msg.Author == nil || msg.Author.ID != 42 || msg.Member == nil || msg.Member.User.ID != 42 {
		t.Fatalf("expected role trigger message authored by target, got %+v", msg)
	}
	if msg.GuildID != 1 || msg.ChannelID != 2 || msg.Member.GuildID != 1 {
		t.Fatalf("unexpected guild/channel on role trigger message: %+v", msg)
	}

	target.Member = nil
	msg = roleTriggerMessage(1, 2, target)
	if msg.Author == nil || msg.Author.ID != 42 || msg.Member == nil || msg.Member.GuildID != 1 {
		t.Fatalf("expected fallback member for target without member data, got %+v", msg)
	}
}
