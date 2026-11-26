package templates

import (
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func testInteractionToken(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("interaction:" + strconv.FormatInt(id, 10) + ":secret"))
}

func TestTokenArgRestrictsToGuild(t *testing.T) {
	const guildID, otherGuildID = 100, 200
	recorded := map[int64]int64{1: guildID, 2: otherGuildID}

	orig := interactionInGuild
	interactionInGuild = func(interactionID, gID int64) bool {
		g, ok := recorded[interactionID]
		return ok && g == gID
	}
	defer func() { interactionInGuild = orig }()

	current := testInteractionToken(3)
	ctx := &Context{
		GS: &dstate.GuildSet{GuildState: dstate.GuildState{ID: guildID}},
		CurrentFrame: &ContextFrame{Interaction: &CustomCommandInteraction{
			Interaction: &discordgo.Interaction{ID: 3, Token: current},
		}},
	}

	cases := []struct {
		name   string
		token  any
		wantOK bool
	}{
		{"current interaction", current, true},
		{"nil falls back to current", nil, true},
		{"same guild", testInteractionToken(1), true},
		{"other guild", testInteractionToken(2), false},
		{"unknown interaction", testInteractionToken(4), false},
		{"garbage", "not a token", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, token := ctx.tokenArg(c.token)
			if (token != "") != c.wantOK {
				t.Fatalf("tokenArg(%v) = %q, want accepted=%v", c.token, token, c.wantOK)
			}
		})
	}
}

func TestFindRoleRejectsForeignRoleObjects(t *testing.T) {
	ctx := &Context{GS: &dstate.GuildSet{
		GuildState: dstate.GuildState{ID: 1},
		Roles:      []discordgo.Role{{ID: 10, Name: "local"}},
	}}

	if r := ctx.FindRole(&discordgo.Role{ID: 10}, acceptRoleObject); r == nil || r.Name != "local" {
		t.Fatalf("expected local role from state, got %v", r)
	}
	if r := ctx.FindRole(discordgo.Role{ID: 10}, acceptRoleObject); r == nil || r.Name != "local" {
		t.Fatalf("expected local role from state, got %v", r)
	}
	if r := ctx.FindRole(&discordgo.Role{ID: 99, Name: "foreign"}, acceptRoleObject); r != nil {
		t.Fatalf("expected foreign role pointer to be rejected, got %v", r)
	}
	if r := ctx.FindRole(discordgo.Role{ID: 99, Name: "foreign"}, acceptRoleObject); r != nil {
		t.Fatalf("expected foreign role value to be rejected, got %v", r)
	}
	if r := ctx.FindRole((*discordgo.Role)(nil), acceptRoleObject); r != nil {
		t.Fatalf("expected nil role to be rejected, got %v", r)
	}
}

func TestDisabledContextFuncsAppliesToDirectRegistrations(t *testing.T) {
	RegisterSetupFunc(func(ctx *Context) {
		ctx.ContextFuncs["testDirectlyRegistered"] = func() string { return "" }
	})

	ctx := NewContext(nil, nil, nil)
	ctx.DisabledContextFuncs = []string{"testDirectlyRegistered", "sendDM"}
	ctx.setupContextFuncs()

	for _, name := range ctx.DisabledContextFuncs {
		if _, ok := ctx.ContextFuncs[name]; ok {
			t.Errorf("disabled context func %q is still registered", name)
		}
	}

	enabled := NewContext(nil, nil, nil)
	enabled.setupContextFuncs()
	if _, ok := enabled.ContextFuncs["testDirectlyRegistered"]; !ok {
		t.Errorf("directly registered func missing when not disabled")
	}
}
