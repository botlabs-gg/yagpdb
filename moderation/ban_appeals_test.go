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
	config := &Config{ActionChannel: 1, BanAppealsEnabled: true}
	if config.BanAppealsReviewChannel() != 1 {
		t.Error("did not fall back to the modlog channel")
	}

	config.BanAppealsChannel = 2
	if config.BanAppealsReviewChannel() != 2 {
		t.Error("appeals channel was not preferred")
	}

	if (&Config{BanAppealsEnabled: true}).BanAppealsOpen() {
		t.Error("appeals open without any channel to post them in")
	}
}
