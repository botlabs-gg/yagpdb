package moderation

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestParseBanAppealModalCustomID(t *testing.T) {
	guildID, messageID, err := parseBanAppealModalCustomID(banAppealModalCustomIDPrefix + "123_456")
	if err != nil || guildID != 123 || messageID != 456 {
		t.Errorf("got %d, %d, %v, want 123, 456, nil", guildID, messageID, err)
	}

	if _, _, err := parseBanAppealModalCustomID(banAppealModalCustomIDPrefix + "123"); err == nil {
		t.Error("missing message id was accepted")
	}
}

func TestBanAppealModalText(t *testing.T) {
	data := discordgo.ModalSubmitInteractionData{
		Components: []discordgo.TopLevelComponent{
			&discordgo.TextDisplay{Content: "info"},
			&discordgo.Label{Component: &discordgo.TextInput{CustomID: banAppealModalInputID, Value: "  please unban me  "}},
		},
	}

	if got := banAppealModalText(data); got != "please unban me" {
		t.Errorf("got %q", got)
	}
}

func TestBanAppealsReviewChannel(t *testing.T) {
	defer func(saved func(int64) bool) { isPremiumGuild = saved }(isPremiumGuild)
	premiumGuild := false
	isPremiumGuild = func(int64) bool { return premiumGuild }

	config := &Config{ActionChannel: 1, BanAppealsChannel: 2, BanAppealsEnabled: true}
	if config.BanAppealsReviewChannel() != 1 {
		t.Error("free server did not use the modlog channel")
	}

	premiumGuild = true
	if config.BanAppealsReviewChannel() != 2 {
		t.Error("premium server's appeals channel was not used")
	}

	if (&Config{BanAppealsEnabled: true, BanAppealsChannel: 2}).BanAppealsReviewChannel() != 2 {
		t.Error("premium server needs no modlog channel when it has an appeals channel")
	}

	premiumGuild = false
	if (&Config{BanAppealsEnabled: true, BanAppealsChannel: 2}).BanAppealsOpen() {
		t.Error("appeals open on a free server without a modlog channel")
	}
}

func TestBanReasonForBannedUser(t *testing.T) {
	cases := map[string]string{
		"destroy_0001: bad bad boy":   "bad bad boy",
		"Old Name#1234: spamming":     "spamming",
		"mod.name: ":                  "",
		"Triggered rule: Invite spam": "Triggered rule: Invite spam",
		"Posting links":               "Posting links",
	}

	for reason, want := range cases {
		if got := BanReasonForBannedUser(reason); got != want {
			t.Errorf("%q: got %q, want %q", reason, got, want)
		}
	}
}

func TestWithBanAppealLink(t *testing.T) {
	const link = "https://example.com/1/appeal"
	cases := []struct{ dm, link, want string }{
		{"You have been banned", link, "You have been banned\n\nYou can appeal your ban on " + link},
		{"Appeal at " + link, link, "Appeal at " + link},
		{"You have been banned", "", "You have been banned"},
		{"  ", link, "  "},
	}

	for _, c := range cases {
		if got := withBanAppealLink(c.dm, c.link); got != c.want {
			t.Errorf("%q: got %q, want %q", c.dm, got, c.want)
		}
	}
}

func TestSplitBanReason(t *testing.T) {
	moderator, reason := splitBanReason("destroy_0001: bad bad boy")
	if moderator != "destroy_0001" || reason != "bad bad boy" {
		t.Errorf("got %q, %q", moderator, reason)
	}

	moderator, reason = splitBanReason("Triggered rule: Invite spam")
	if moderator != "" || reason != "Triggered rule: Invite spam" {
		t.Errorf("got %q, %q", moderator, reason)
	}
}

func TestParseBanAppealReviewCustomID(t *testing.T) {
	if unban, id, ok := parseBanAppealReviewCustomID(banAppealUnbanCustomIDPrefix + "42"); !ok || !unban || id != 42 {
		t.Errorf("unban: got %t %d %t", unban, id, ok)
	}
	if unban, id, ok := parseBanAppealReviewCustomID(banAppealRejectCustomIDPrefix + "7"); !ok || unban || id != 7 {
		t.Errorf("reject: got %t %d %t", unban, id, ok)
	}
	if _, _, ok := parseBanAppealReviewCustomID("templates-1"); ok {
		t.Error("foreign custom id was claimed")
	}
}
