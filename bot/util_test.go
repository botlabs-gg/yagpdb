package bot

import (
	"fmt"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func TestMemberHighestRole(t *testing.T) {
	gs := &dstate.GuildSet{
		GuildState: dstate.GuildState{},
		Roles: []discordgo.Role{
			{ID: 10, Position: 10},
			{ID: 5, Position: 5},
			{ID: 100, Position: 1},
			{ID: 102, Position: 1},
		},
	}

	cases := []struct {
		Roles   []int64
		Highest int64
	}{
		{Roles: []int64{100, 5, 10}, Highest: 10},
		{Roles: []int64{102, 100}, Highest: 100},
		{Roles: []int64{5, 102}, Highest: 5},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			ms := &dstate.MemberState{
				Member: &dstate.MemberFields{
					Roles: v.Roles,
				},
			}

			result := MemberHighestRole(gs, ms)
			if result.ID != v.Highest {
				t.Errorf("incorrect result, got %d, expected %d", result.ID, v.Highest)
			}
		})
	}
}

func TestIsMemberAbove(t *testing.T) {
	gs := &dstate.GuildSet{
		GuildState: dstate.GuildState{
			OwnerID: 99,
		},
		Roles: []discordgo.Role{
			{ID: 10, Position: 10},
			{ID: 5, Position: 5},
			{ID: 100, Position: 1},
			{ID: 102, Position: 1},
		},
	}

	cases := []struct {
		M1 []int64
		M2 []int64

		Above bool
	}{
		{M1: []int64{100, 5}, M2: []int64{10, 100}, Above: false},
		{M1: []int64{100, 5, 10}, M2: []int64{10, 100}, Above: false},
		{M1: []int64{100, 5, 10}, M2: []int64{100}, Above: true},
		{M1: []int64{100, 102}, M2: []int64{102}, Above: true},
		{M1: []int64{100}, M2: []int64{100}, Above: false},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			ms1 := &dstate.MemberState{
				Member: &dstate.MemberFields{
					Roles: v.M1,
				},
			}

			ms2 := &dstate.MemberState{
				Member: &dstate.MemberFields{
					Roles: v.M2,
				},
			}

			result := IsMemberAbove(gs, ms1, ms2)
			if result != v.Above {
				t.Errorf("incorrect result, got %t, expected %t", result, v.Above)
			}
		})
	}
}

func TestValidateDMComponents(t *testing.T) {
	linkButton := &discordgo.Button{Style: discordgo.LinkButton, URL: "https://example.com", Label: "Verify"}
	customButton := &discordgo.Button{Style: discordgo.PrimaryButton, CustomID: "x", Label: "Press"}

	cases := []struct {
		name       string
		components []discordgo.TopLevelComponent
		wantErr    bool
	}{
		{"link button row", []discordgo.TopLevelComponent{&discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{linkButton}}}, false},
		{"custom button row", []discordgo.TopLevelComponent{&discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{customButton}}}, true},
		{"mixed row", []discordgo.TopLevelComponent{discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{linkButton, customButton}}}, true},
		{"link button in container", []discordgo.TopLevelComponent{&discordgo.Container{Components: []discordgo.TopLevelComponent{
			&discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{linkButton}},
		}}}, false},
		{"link button accessory", []discordgo.TopLevelComponent{discordgo.Section{Accessory: linkButton}}, false},
		{"custom button accessory", []discordgo.TopLevelComponent{&discordgo.Section{Accessory: customButton}}, true},
	}

	for _, c := range cases {
		err := ValidateDMComponents(c.components)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: got err %v, want error %t", c.name, err, c.wantErr)
		}
	}
}

func TestDMComponentsRowIsLast(t *testing.T) {
	linkRow := func() discordgo.TopLevelComponent {
		return &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
			&discordgo.Button{Style: discordgo.LinkButton, URL: "https://example.com", Label: "Link"},
		}}
	}
	isDMRow := func(c discordgo.TopLevelComponent) bool {
		row, ok := c.(discordgo.ActionsRow)
		return ok && len(row.Components) > 0 && row.Components[0].(discordgo.Button).CustomID == fmt.Sprintf("%s%d", DMServerInfoCustomIDPrefix, 1)
	}

	legacy := []discordgo.TopLevelComponent{linkRow(), linkRow(), linkRow(), linkRow(), linkRow()}
	got := DMComponents(1, legacy, 0)
	if len(got) != MaxLegacyTopLevelComponents || !isDMRow(got[len(got)-1]) {
		t.Errorf("legacy: want %d rows ending with the dm row, got %d", MaxLegacyTopLevelComponents, len(got))
	}

	// 18 rows of 2 plus a trailing text display is 37, leaving no room for the dm row.
	var v2 []discordgo.TopLevelComponent
	for range 18 {
		v2 = append(v2, linkRow())
	}
	text := discordgo.TextDisplay{Content: "body"}
	v2 = append(v2, text)

	got = DMComponents(1, v2, discordgo.MessageFlagsIsComponentsV2)
	if !isDMRow(got[len(got)-1]) {
		t.Fatal("v2: dm row is not last")
	}
	if got[len(got)-2] != discordgo.TopLevelComponent(text) {
		t.Error("v2: body was clipped instead of an action row")
	}
	if countComponents(got) > MaxComponentsV2Total {
		t.Errorf("v2: %d components exceeds the limit", countComponents(got))
	}
}

func TestOwnVerificationLinks(t *testing.T) {
	common.ConfHost.LoadedValue = "yagpdb.xyz"

	cases := []struct {
		name    string
		msg     *discordgo.Message
		genuine bool
	}{
		{"short link in embed", &discordgo.Message{Embeds: []*discordgo.MessageEmbed{{Description: "Verify: https://yagpdb.xyz/v/AbCdEfGhIjKlMnOp"}}}, true},
		{"long link in content", &discordgo.Message{Content: "https://yagpdb.xyz/public/1/verify/2/AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEf"}, true},
		{"link button", &discordgo.Message{Components: []discordgo.TopLevelComponent{&discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{
			&discordgo.Button{Style: discordgo.LinkButton, URL: "https://yagpdb.xyz/v/AbCdEfGhIjKlMnOp", Label: "Verify"},
		}}}}, true},
		{"scam beside a genuine link", &discordgo.Message{Content: "https://yagpdb.xyz/v/AbCdEfGhIjKlMnOp and https://free-nitro.example"}, false},
		{"scam in a container", &discordgo.Message{
			Content: "https://yagpdb.xyz/v/AbCdEfGhIjKlMnOp",
			Components: []discordgo.TopLevelComponent{&discordgo.Container{Components: []discordgo.TopLevelComponent{
				&discordgo.TextDisplay{Content: "https://free-nitro.example"},
			}}},
		}, false},
		{"look-alike host", &discordgo.Message{Content: "https://yagpdb.xyz.example/v/AbCdEfGhIjKlMnOp"}, false},
		{"other page on our host", &discordgo.Message{Content: "https://yagpdb.xyz/manage/1"}, false},
		{"no links", &discordgo.Message{Content: "hello"}, false},
	}

	for _, c := range cases {
		genuine, err := onlyVerificationLinks(c.msg)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}

		if genuine != c.genuine {
			t.Errorf("%s: got genuine %t, want %t", c.name, genuine, c.genuine)
		}
	}
}

func TestDMComponentsExtraButtons(t *testing.T) {
	extra := discordgo.Button{Label: "Appeal", Style: discordgo.SuccessButton, CustomID: "DM_EXTRA_1"}

	got := DMComponents(1, nil, 0, extra)
	row := got[len(got)-1].(discordgo.ActionsRow)

	info := row.Components[0].(discordgo.Button)
	if info.CustomID != fmt.Sprintf("%s%d", DMServerInfoCustomIDPrefix, 1) {
		t.Error("info button is no longer first")
	}
	if info.Emoji == nil || info.Emoji.Name != "ℹ️" {
		t.Error("info button should show ℹ️")
	}
	for _, c := range row.Components[:len(row.Components)-1] {
		if label := c.(discordgo.Button).Label; label != "" {
			t.Errorf("dm buttons should only show their emoji, got label %q", label)
		}
	}
	if last := row.Components[len(row.Components)-1].(discordgo.Button); last.CustomID != extra.CustomID {
		t.Errorf("extra button not appended, last button is %q", last.CustomID)
	}
}

func TestRegisteredDMInteractionHandler(t *testing.T) {
	defer func(saved []dmInteractionHandler) { dmInteractionHandlers = saved }(dmInteractionHandlers)
	dmInteractionHandlers = nil

	called := ""
	RegisterDMInteractionHandler("DM_TEST_", func(evt *eventsystem.EventData) { called = "test" })

	button := &discordgo.InteractionCreate{Interaction: discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: "DM_TEST_123"},
	}}
	handler, ok := registeredDMInteractionHandler(button)
	if !ok {
		t.Fatal("registered prefix was not matched")
	}
	handler(nil)
	if called != "test" {
		t.Error("wrong handler returned")
	}

	other := &discordgo.InteractionCreate{Interaction: discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: "DM_123"},
	}}
	if _, ok := registeredDMInteractionHandler(other); ok {
		t.Error("unregistered prefix was matched")
	}
}
