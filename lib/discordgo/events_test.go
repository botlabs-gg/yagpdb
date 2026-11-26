package discordgo

import (
	"encoding/json"
	"testing"
)

func TestMessageReactionAddMember(t *testing.T) {
	raw := `{"user_id":"1","message_id":"2","channel_id":"3","guild_id":"4","emoji":{"name":"x"},"member":{"user":{"id":"1"},"roles":["5"]}}`

	var evt MessageReactionAdd
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.MessageReaction == nil || evt.UserID != 1 || evt.GuildID != 4 {
		t.Fatalf("reaction not decoded: %+v", evt.MessageReaction)
	}
	if evt.Member == nil || evt.Member.User == nil || evt.Member.User.ID != 1 || len(evt.Member.Roles) != 1 || evt.Member.Roles[0] != 5 {
		t.Fatalf("member not decoded: %+v", evt.Member)
	}
}
