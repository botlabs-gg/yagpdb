package automod_legacy

import (
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestLinksRuleIncludeExtraText(t *testing.T) {
	m := &discordgo.Message{
		Content: "check this out",
		Embeds:  []*discordgo.MessageEmbed{{Description: "more at https://evil.example.com"}},
	}

	if (&LinksRule{}).containsLink(m) {
		t.Error("expected a link only in an embed to be ignored by default")
	}

	rule := &LinksRule{IncludeExtraTextOption: IncludeExtraTextOption{IncludeExtraText: true}}
	if !rule.containsLink(m) {
		t.Error("expected a link in an embed to be found with IncludeExtraText")
	}
}

func TestIncludeExtraTextOptionDefaultsToContent(t *testing.T) {
	m := &discordgo.Message{
		Content:          "hello",
		MessageSnapshots: []*discordgo.MessageSnapshot{{Message: &discordgo.Message{Content: "forwarded"}}},
	}

	got := IncludeExtraTextOption{}.texts(m)
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("expected only the message content by default, got %q", got)
	}
}

func TestPunishmentFor(t *testing.T) {
	rule := BaseRule{MuteAfter: 2, TimeoutAfter: 3, KickAfter: 5, BanAfter: 7}

	cases := map[int]Punishment{1: PunishNone, 2: PunishMute, 3: PunishTimeout, 4: PunishTimeout, 5: PunishKick, 7: PunishBan}
	for violations, want := range cases {
		if got := rule.punishmentFor(violations); got != want {
			t.Errorf("%d violations: got %v, want %v", violations, got, want)
		}
	}

	if got := (BaseRule{TimeoutAfter: 1}).punishmentFor(1); got != PunishTimeout {
		t.Errorf("expected a timeout on its own to apply, got %v", got)
	}
}

func TestTimeoutDurationDefault(t *testing.T) {
	if got := (BaseRule{}).GetTimeoutDuration(); got != 10*time.Minute {
		t.Errorf("expected the default timeout duration when unset, got %v", got)
	}
	if got := (BaseRule{TimeoutDuration: 60}).GetTimeoutDuration(); got != time.Hour {
		t.Errorf("got %v, want 1h", got)
	}
}
