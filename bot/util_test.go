package bot

import (
	"fmt"
	"testing"

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
