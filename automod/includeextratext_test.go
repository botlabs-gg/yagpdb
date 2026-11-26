package automod

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func linkPreviewMessage(previewText string) *discordgo.Message {
	return &discordgo.Message{
		Content: "check this out",
		Embeds: []*discordgo.MessageEmbed{{
			Type: "components",
			Components: []discordgo.TopLevelComponent{
				&discordgo.Container{Components: []discordgo.TopLevelComponent{
					&discordgo.TextDisplay{Content: previewText},
				}},
			},
		}},
	}
}

func checkTrigger(t *testing.T, trigger MessageTrigger, data any, m *discordgo.Message) bool {
	t.Helper()
	triggered, err := trigger.CheckMessage(&TriggerContext{Data: data}, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	return triggered
}

func TestAnyLinkIncludeExtraText(t *testing.T) {
	m := linkPreviewMessage("join us at https://evil.example.com")

	if checkTrigger(t, &AnyLinkTrigger{}, &IncludeExtraTextTriggerData{}, m) {
		t.Error("expected a link only in an embed to be ignored by default")
	}
	if !checkTrigger(t, &AnyLinkTrigger{}, &IncludeExtraTextTriggerData{IncludeExtraText: true}, m) {
		t.Error("expected a link in an embed to trigger with IncludeExtraText")
	}
}

func TestMessageRegexIncludeExtraText(t *testing.T) {
	if RegexCache == nil {
		RegexCache = common.NewUserRegexCache()
	}
	m := linkPreviewMessage("forbidden phrase")

	match := &MessageRegexTrigger{BaseRegexTrigger{Inverse: false}}
	if checkTrigger(t, match, &BaseRegexTriggerData{Regex: "forbidden"}, m) {
		t.Error("expected embed text to be ignored by default")
	}
	if !checkTrigger(t, match, &BaseRegexTriggerData{Regex: "forbidden", IncludeExtraText: true}, m) {
		t.Error("expected embed text to match with IncludeExtraText")
	}

	inverse := &MessageRegexTrigger{BaseRegexTrigger{Inverse: true}}
	if checkTrigger(t, inverse, &BaseRegexTriggerData{Regex: "check", IncludeExtraText: true}, m) {
		t.Error("expected the inverse regex trigger to only check message content")
	}
	for _, s := range inverse.UserSettings() {
		if s.Key == "IncludeExtraText" {
			t.Error("expected the inverse regex trigger not to offer IncludeExtraText")
		}
	}
}

func TestIncludeExtraTextMergeDuplicates(t *testing.T) {
	merged := (&AnyLinkTrigger{}).MergeDuplicates([]any{
		&IncludeExtraTextTriggerData{},
		&IncludeExtraTextTriggerData{IncludeExtraText: true},
	}).(*IncludeExtraTextTriggerData)
	if !merged.IncludeExtraText {
		t.Error("expected IncludeExtraText to survive merging duplicates")
	}
}

func TestAnyLinkIncludesPolls(t *testing.T) {
	m := &discordgo.Message{Poll: &discordgo.Poll{
		Question: discordgo.PollMedia{Text: "best site?"},
		Answers:  []discordgo.PollAnswer{{PollMedia: discordgo.PollMedia{Text: "https://evil.example.com"}}},
	}}

	if checkTrigger(t, &AnyLinkTrigger{}, &IncludeExtraTextTriggerData{}, m) {
		t.Error("expected a link only in a poll answer to be ignored by default")
	}
	if !checkTrigger(t, &AnyLinkTrigger{}, &IncludeExtraTextTriggerData{IncludeExtraText: true}, m) {
		t.Error("expected a link in a poll answer to trigger with IncludeExtraText")
	}
}
