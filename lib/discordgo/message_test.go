package discordgo

import (
	"encoding/json"
	"testing"
)

func TestContentWithMoreMentionsReplaced(t *testing.T) {
	s := &Session{StateEnabled: true, State: NewState()}

	user := &User{
		ID:       1,
		Username: "User Name",
	}

	s.StateEnabled = true
	s.State.GuildAdd(&Guild{ID: 10})
	s.State.RoleAdd(10, &Role{
		ID:          20,
		Name:        "Role Name",
		Mentionable: true,
	})
	s.State.MemberAdd(&Member{
		User:    user,
		Nick:    "User Nick",
		GuildID: 10,
	})
	s.State.ChannelAdd(&Channel{
		Name:    "Channel Name",
		GuildID: 10,
		ID:      30,
	})

	m := &Message{
		Content:      "<@&20> <@!1> <@1> <#30>",
		ChannelID:    30,
		MentionRoles: []int64{20},
		Mentions:     []*User{user},
	}

	if result, _ := m.ContentWithMoreMentionsReplaced(s); result != "@Role Name @User Nick @User Name #Channel Name" {
		t.Error(result)
	}
}

func TestComponentEmbedUnmarshal(t *testing.T) {
	raw := `{"id":"1","flags":32768,"components":[{"type":17,"components":[{"type":10,"content":"v2 body"}]}],
		"embeds":[{"type":"components","url":"https://yagpdb.xyz","components":[{"type":17,"accent_color":5793266,"components":[
			{"type":9,"components":[{"type":10,"content":"# YAGPDB"}],"accessory":{"type":11,"media":{"url":"https://yagpdb.xyz/a.png"}}},
			{"type":14},
			{"type":10,"content":"Features"},
			{"type":1,"components":[{"type":2,"style":5,"label":"Docs","url":"https://help.yagpdb.xyz"}]}
		]}]}]}`

	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}

	if m.Content != "" {
		t.Errorf("expected content to match what discord sent, got %q", m.Content)
	}
	if got := m.TextContents(); len(got) != 1 || got[0] != "v2 body" {
		t.Errorf("unexpected message text contents: %q", got)
	}

	embed := m.Embeds[0]
	if len(embed.Components) != 1 {
		t.Fatalf("expected 1 top level component, got %d", len(embed.Components))
	}
	container, ok := embed.Components[0].(*Container)
	if !ok || container.AccentColor != 5793266 || len(container.Components) != 4 {
		t.Fatalf("unexpected container: %#v", embed.Components[0])
	}

	got := embed.TextContents()
	if len(got) != 2 || got[0] != "# YAGPDB" || got[1] != "Features" {
		t.Errorf("unexpected text contents: %q", got)
	}

	sent := ValidateComplexMessageEmbeds([]*MessageEmbed{embed})
	if len(sent) != 1 || sent[0].Components != nil || len(embed.Components) != 1 {
		t.Errorf("expected components stripped from a copy when sending")
	}
}

func TestGetAllTextContents(t *testing.T) {
	raw := `{"id":"1","content":"look at this",
		"poll":{"question":{"text":"poll question"},"answers":[{"answer_id":1,"poll_media":{"text":"answer one"}},{"answer_id":2,"poll_media":{"text":"answer two","emoji":{"name":"🔥"}}}],"expiry":"2026-11-18T00:00:00+00:00","allow_multiselect":false,"layout_type":1},
		"components":[{"type":17,"components":[
			{"type":10,"content":"cv2 text"},
			{"type":1,"components":[{"type":2,"style":5,"label":"Open","url":"https://button.example"}]}
		]}],
		"embeds":[
			{"type":"rich","title":"embed title","description":"embed description","author":{"name":"author name"},"footer":{"text":"footer text"},"fields":[{"name":"field name","value":"field value"}]},
			{"type":"components","url":"https://preview.example","components":[{"type":17,"components":[
				{"type":9,"components":[{"type":10,"content":"preview text"}],"accessory":{"type":2,"style":5,"label":"Visit","url":"https://accessory.example"}}
			]}]}
		],
		"message_snapshots":[{"message":{"content":"forwarded","embeds":[{"type":"rich","description":"forwarded embed"}]}}]}`

	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"look at this", "forwarded",
		"poll question", "answer one", "answer two",
		"cv2 text", "Open", "https://button.example",
		"embed title", "embed description", "author name", "footer text", "field name", "field value",
		"https://preview.example", "preview text", "Visit", "https://accessory.example",
		"forwarded embed",
	}

	got := m.GetAllTextContents()
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestGetAllTextContentsTypedNilComponents(t *testing.T) {
	m := &Message{
		Content: "hi",
		Components: []TopLevelComponent{
			(*Container)(nil), (*ActionsRow)(nil), (*Section)(nil), (*TextDisplay)(nil),
			&Container{Components: []TopLevelComponent{(*TextDisplay)(nil), &ActionsRow{Components: []InteractiveComponent{(*Button)(nil)}}, &Section{Accessory: (*Button)(nil)}}},
		},
	}

	got := m.GetAllTextContents()
	if len(got) != 1 || got[0] != "hi" {
		t.Fatalf("got %q, want [hi]", got)
	}
}
