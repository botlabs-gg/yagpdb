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
