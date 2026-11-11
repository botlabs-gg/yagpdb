package automod

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func checkFlaggedMedia(t *testing.T, data *FlaggedMediaTriggerData, m *discordgo.Message) bool {
	t.Helper()
	triggered, err := (&FlaggedMediaTrigger{}).CheckMessage(&TriggerContext{Data: data}, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	return triggered
}

func TestFlaggedMediaTrigger(t *testing.T) {
	all := &FlaggedMediaTriggerData{Explicit: true, Gore: true, SelfHarm: true}

	explicit := &discordgo.Message{Attachments: []*discordgo.MessageAttachment{{Flags: discordgo.MessageAttachmentFlagContainsExplicitContent}}}
	if !checkFlaggedMedia(t, all, explicit) {
		t.Fatal("expected a flagged attachment to trigger")
	}

	if checkFlaggedMedia(t, &FlaggedMediaTriggerData{Gore: true}, explicit) {
		t.Fatal("expected explicit content not to trigger when only gore is checked")
	}

	animated := &discordgo.Message{Attachments: []*discordgo.MessageAttachment{{Flags: discordgo.MessageAttachmentFlagIsAnimated | discordgo.MessageAttachmentFlagIsSpoiler}}}
	if checkFlaggedMedia(t, all, animated) {
		t.Fatal("expected flags other than content scanning not to trigger")
	}

	embed := &discordgo.Message{Embeds: []*discordgo.MessageEmbed{{Flags: discordgo.MessageEmbedFlagContainsGoreContent}}}
	if !checkFlaggedMedia(t, all, embed) {
		t.Fatal("expected a flagged embed to trigger")
	}

	forwarded := &discordgo.Message{MessageSnapshots: []*discordgo.MessageSnapshot{{Message: explicit}}}
	if !checkFlaggedMedia(t, all, forwarded) {
		t.Fatal("expected flagged media in a forwarded message to trigger")
	}
}

func TestFlaggedMediaMergeDuplicates(t *testing.T) {
	merged := (&FlaggedMediaTrigger{}).MergeDuplicates([]any{
		&FlaggedMediaTriggerData{Explicit: true},
		&FlaggedMediaTriggerData{SelfHarm: true},
	}).(*FlaggedMediaTriggerData)

	if !merged.Explicit || merged.Gore || !merged.SelfHarm {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}
