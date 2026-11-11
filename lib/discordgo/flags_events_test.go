package discordgo

import (
	"encoding/json"
	"testing"
)

func TestContentScanFlagsDecode(t *testing.T) {
	var m Message
	payload := `{"attachments":[{"id":"1","flags":96}],"embeds":[{"type":"image","flags":128}]}`
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatal(err)
	}

	flags := m.Attachments[0].Flags
	if flags&MessageAttachmentFlagIsAnimated == 0 || flags&MessageAttachmentFlagContainsGoreContent == 0 || flags&MessageAttachmentFlagContainsExplicitContent != 0 {
		t.Fatalf("unexpected attachment flags: %b", flags)
	}

	if m.Embeds[0].Flags&MessageEmbedFlagContainsSelfHarmContent == 0 {
		t.Fatalf("unexpected embed flags: %b", m.Embeds[0].Flags)
	}
}

func TestVoiceChannelEventsDecode(t *testing.T) {
	var status VoiceChannelStatusUpdate
	if err := json.Unmarshal([]byte(`{"id":"2","guild_id":"1","status":"gaming"}`), &status); err != nil {
		t.Fatal(err)
	}
	if status.ID != 2 || status.GuildID != 1 || status.Status != "gaming" {
		t.Fatalf("unexpected status update: %+v", status)
	}

	var cleared VoiceChannelStatusUpdate
	if err := json.Unmarshal([]byte(`{"id":"2","guild_id":"1","status":null}`), &cleared); err != nil || cleared.Status != "" {
		t.Fatalf("expected a cleared status, got %+v %v", cleared, err)
	}

	var start VoiceChannelStartTimeStatusUpdate
	if err := json.Unmarshal([]byte(`{"id":"2","guild_id":"1","voice_start_time":1700000000}`), &start); err != nil || start.VoiceStartTime != 1700000000 {
		t.Fatalf("unexpected start time update: %+v %v", start, err)
	}
}
