package common

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
)

func TestDiscordInviteRegex(t *testing.T) {
	testcases := []struct {
		input    string
		inviteID string
	}{
		{input: "https://discordapp.com/invite/FPxNX2", inviteID: "FPxNX2"},
		{input: "discordapp.com/developers/docs/reference#message-formatting", inviteID: ""},
		{input: "https://discord.gg/FPxNX2", inviteID: "FPxNX2"},
		{input: "https://discord.gg/landfall", inviteID: "landfall"},
		{input: "HElllo there", inviteID: ""},
		{input: "Jajajaj", inviteID: ""},
	}

	for _, v := range testcases {
		t.Run("Case "+v.input, func(t *testing.T) {
			v.input = confusables.NormalizeQueryEncodedText(v.input)
			matches := DiscordInviteSource.Regex.FindAllStringSubmatch(v.input, -1)
			if len(matches) < 1 && v.inviteID != "" {
				t.Error("No matches")
			}

			if len(matches) < 1 {
				return
			}

			if len(matches[0]) < 2 {
				if v.inviteID != "" {
					t.Error("ID Not found!")
				}

				return
			}

			id := matches[0][2]
			if id != v.inviteID {
				t.Errorf("Found ID %q, expected %q", id, v.inviteID)
			}
		})
	}
}
