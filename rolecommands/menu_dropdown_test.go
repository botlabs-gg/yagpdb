package rolecommands

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
	"github.com/volatiletech/null/v8"
)

func TestParseOptionEmoji(t *testing.T) {
	tests := []struct {
		input    string
		id       int64
		unicode  string
		animated bool
		err      bool
	}{
		{input: ""},
		{input: "🔥", unicode: "🔥"},
		{input: " 🇺🇸 ", unicode: "🇺🇸"},
		{input: "<:yag:123456789012345678>", id: 123456789012345678},
		{input: "<a:party:123456789012345678>", id: 123456789012345678, animated: true},
		{input: "fire", err: true},
		{input: ":fire:", err: true},
		{input: "<:yag:12>", err: true},
	}

	for _, tt := range tests {
		id, unicode, animated, err := parseOptionEmoji(tt.input)
		if (err != nil) != tt.err || id != tt.id || unicode != tt.unicode || animated != tt.animated {
			t.Errorf("parseOptionEmoji(%q) = %d, %q, %v, %v", tt.input, id, unicode, animated, err)
		}
	}
}

func TestParseMessageLink(t *testing.T) {
	g, c, m, err := parseMessageLink("https://ptb.discord.com/channels/1/2/3")
	if err != nil || g != 1 || c != 2 || m != 3 {
		t.Errorf("got %d %d %d %v", g, c, m, err)
	}

	for _, link := range []string{"", "https://discord.com/channels/1/2", "https://evil.com/channels/1/2/3", "https://discord.com/channels/@me/2/3"} {
		if _, _, _, err := parseMessageLink(link); err == nil {
			t.Errorf("expected %q to be rejected", link)
		}
	}
}

func testDropdownMenu(mode int64) *models.RoleMenu {
	group := &models.RoleGroup{ID: 1, Name: "Colors", Mode: mode}
	menu := &models.RoleMenu{MessageID: 10, Kind: RoleMenuKindDropdown, RoleGroupID: null.Int64From(1)}
	menu.R = menu.R.NewStruct()
	menu.R.RoleGroup = group

	for i, name := range []string{"Red", "Blue"} {
		option := &models.RoleMenuOption{ID: int64(i + 1), RoleCommandID: null.Int64From(int64(100 + i))}
		option.R = option.R.NewStruct()
		option.R.RoleCommand = &models.RoleCommand{ID: int64(100 + i), Name: name, Position: int64(i)}
		menu.R.RoleMenuOptions = append(menu.R.RoleMenuOptions, option)
	}
	menu.R.RoleMenuOptions[1].Label = "Blue role"
	menu.R.RoleMenuOptions[1].EmojiID = 5
	return menu
}

func TestRoleMenuDropdownRow(t *testing.T) {
	row := roleMenuDropdownRow(testDropdownMenu(GroupModeNone))
	menu := row.Components[0].(discordgo.SelectMenu)

	if menu.CustomID != roleMenuDropdownCustomID || menu.Placeholder != defaultDropdownPlaceholder || menu.MaxValues != 2 {
		t.Fatalf("unexpected menu %+v", menu)
	}
	if len(menu.Options) != 2 || menu.Options[0].Label != "Red" || menu.Options[0].Value != "100" || menu.Options[0].Emoji != nil {
		t.Fatalf("unexpected first option %+v", menu.Options[0])
	}
	if menu.Options[1].Label != "Blue role" || menu.Options[1].Emoji == nil || menu.Options[1].Emoji.ID != 5 {
		t.Fatalf("unexpected second option %+v", menu.Options[1])
	}

	single := roleMenuDropdownRow(testDropdownMenu(GroupModeSingle)).Components[0].(discordgo.SelectMenu)
	if single.MaxValues != 1 {
		t.Errorf("single mode menu allows %d picks", single.MaxValues)
	}
}

func TestWithRoleMenuDropdown(t *testing.T) {
	other := discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{discordgo.Button{Label: "Other", CustomID: "templates-x"}}}
	stale := roleMenuDropdownRow(testDropdownMenu(GroupModeNone))

	components := withRoleMenuDropdown([]discordgo.TopLevelComponent{other, &stale}, testDropdownMenu(GroupModeSingle))
	if len(components) != 2 || isRoleMenuDropdownRow(components[0]) || !isRoleMenuDropdownRow(components[1]) {
		t.Fatalf("unexpected components %+v", components)
	}
	if components[1].(discordgo.ActionsRow).Components[0].(discordgo.SelectMenu).MaxValues != 1 {
		t.Error("the dropdown was not replaced")
	}

	if stripped := withoutRoleMenuDropdown(components); len(stripped) != 1 || isRoleMenuDropdownRow(stripped[0]) {
		t.Errorf("unexpected stripped components %+v", stripped)
	}
}

func TestRolesAfterToggle(t *testing.T) {
	settings := &CommonRoleSettings{RoleId: 3, ParentGroupMode: GroupModeNone}
	if roles := rolesAfterToggle(t.Context(), settings, []int64{1, 2}, true); len(roles) != 3 || roles[2] != 3 {
		t.Errorf("giving: %v", roles)
	}
	if roles := rolesAfterToggle(t.Context(), settings, []int64{1, 3}, false); len(roles) != 1 || roles[0] != 1 {
		t.Errorf("taking: %v", roles)
	}
}
