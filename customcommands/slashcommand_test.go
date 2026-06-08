package customcommands

import (
	"encoding/json"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/volatiletech/null/v8"
)

func TestSlashCommandOptionsSortsRequiredFirst(t *testing.T) {
	cc := &CustomCommand{
		SlashOptions: []SlashCommandOptionForm{
			{Name: "opt_a", Type: "string", Description: "desc a"},
			{Name: "  ", Type: "string"}, // fully empty → dropped
			{Name: "opt_b", Type: "user", Description: "desc b", Required: true},
			{Name: "opt_c", Type: "integer", Description: "desc c"},
		},
	}

	opts := cc.SlashCommandOptions()

	// the blank/empty row must be skipped
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d: %+v", len(opts), opts)
	}

	// required option (opt_b) must be ordered first
	if opts[0].Name != "opt_b" || !opts[0].Required {
		t.Errorf("expected required opt_b first, got %+v", opts[0])
	}

	// names are lowercased and trimmed
	for _, o := range opts {
		if o.Name == "" {
			t.Errorf("unexpected empty option name in %+v", opts)
		}
	}
}

func TestToDBModelAndParseSlashCommandRoundTrip(t *testing.T) {
	cc := &CustomCommand{
		TriggerTypeForm:         "slash_command",
		TriggerType:             CommandTriggerSlash,
		Trigger:                 "MyCommand", // should be lowercased
		SlashCommandDescription: "does a thing",
		SlashOptions: []SlashCommandOptionForm{
			{Name: "target", Type: "user", Description: "the target", Required: true},
			{Name: "count", Type: "integer", Description: "how many"},
		},
		Responses: []string{"hello"},
	}

	dbModel := cc.ToDBModel()

	if dbModel.TextTrigger != "mycommand" {
		t.Errorf("expected lowercased command name, got %q", dbModel.TextTrigger)
	}
	if !dbModel.SlashCommandOptions.Valid {
		t.Fatal("expected slash command options to be set")
	}

	data := parseSlashCommandData(dbModel)
	if data.Description != "does a thing" {
		t.Errorf("unexpected description %q", data.Description)
	}
	if len(data.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(data.Options))
	}
	// required-first ordering preserved through serialization
	if data.Options[0].Name != "target" || !data.Options[0].Required {
		t.Errorf("expected required target option first, got %+v", data.Options[0])
	}
}

func TestSlashCommandOptionsParsesExtraProperties(t *testing.T) {
	cc := &CustomCommand{
		SlashOptions: []SlashCommandOptionForm{
			{Name: "choice", Type: "string_menu", Description: "pick one", Choices: "red\ngreen\n\nblue\n"},
			{Name: "amount", Type: "integer", Description: "how many", MinValue: "1", MaxValue: "10"},
			{Name: "note", Type: "string", Description: "free text", MinLength: "2", MaxLength: "50"},
			{Name: "user", Type: "user", Description: "who"},
		},
	}

	opts := cc.SlashCommandOptions()
	byName := map[string]SlashCommandOption{}
	for _, o := range opts {
		byName[o.Name] = o
	}

	// String with choices → select menu (blank lines dropped)
	if got := byName["choice"].Choices; len(got) != 3 || got[0] != "red" || got[2] != "blue" {
		t.Errorf("unexpected choices: %#v", got)
	}
	// choices option must not also carry length constraints
	if byName["choice"].MinLength != nil || byName["choice"].MaxLength != nil {
		t.Errorf("choice option should not have length constraints")
	}
	// integer min/max value
	if byName["amount"].MinValue == nil || *byName["amount"].MinValue != 1 || byName["amount"].MaxValue == nil || *byName["amount"].MaxValue != 10 {
		t.Errorf("unexpected min/max value: %+v", byName["amount"])
	}
	// free-text string min/max length
	if byName["note"].MinLength == nil || *byName["note"].MinLength != 2 || byName["note"].MaxLength == nil || *byName["note"].MaxLength != 50 {
		t.Errorf("unexpected min/max length: %+v", byName["note"])
	}
	// non-text/number types carry none of the extras
	u := byName["user"]
	if len(u.Choices) != 0 || u.MinValue != nil || u.MaxValue != nil || u.MinLength != nil || u.MaxLength != nil {
		t.Errorf("user option should have no extra properties: %+v", u)
	}
}

func TestBuildSlashCommandRequestMapsExtras(t *testing.T) {
	min := 2
	max := 50
	minV := 1.0
	maxV := 10.0
	payload := slashCommandData{
		Description: "x",
		Options: []SlashCommandOption{
			{Name: "color", Type: int(discordgo.ApplicationCommandOptionString), Description: "c", Choices: []string{"red", "blue"}},
			{Name: "len", Type: int(discordgo.ApplicationCommandOptionString), Description: "l", MinLength: &min, MaxLength: &max},
			{Name: "num", Type: int(discordgo.ApplicationCommandOptionInteger), Description: "n", MinValue: &minV, MaxValue: &maxV},
		},
	}
	b, _ := json.Marshal(payload)
	cc := &models.CustomCommand{TextTrigger: "t", SlashCommandOptions: null.JSONFrom(b)}

	req := buildSlashCommandRequest(cc)
	byName := map[string]*discordgo.ApplicationCommandOption{}
	for _, o := range req.Options {
		byName[o.Name] = o
	}

	if len(byName["color"].Choices) != 2 || byName["color"].Choices[0].Name != "red" || byName["color"].Choices[0].Value != "red" {
		t.Errorf("choices not mapped: %#v", byName["color"].Choices)
	}
	if byName["len"].MinLength == nil || *byName["len"].MinLength != 2 || byName["len"].MaxLength == nil || *byName["len"].MaxLength != 50 {
		t.Errorf("length not mapped: %+v", byName["len"])
	}
	if byName["num"].MinValue == nil || *byName["num"].MinValue != 1 || byName["num"].MaxValue != 10 {
		t.Errorf("value not mapped: %+v", byName["num"])
	}
}

func TestSlashCommandChannelTypes(t *testing.T) {
	cc := &CustomCommand{
		SlashOptions: []SlashCommandOptionForm{
			// valid (text=0, voice=2) plus an invalid value (99) that must be filtered out
			{Name: "chan", Type: "channel", Description: "pick", ChannelTypes: []int{0, 2, 99}},
			// channel types on a non-channel option must be ignored
			{Name: "txt", Type: "string", Description: "t", ChannelTypes: []int{0}},
		},
	}

	opts := cc.SlashCommandOptions()
	byName := map[string]SlashCommandOption{}
	for _, o := range opts {
		byName[o.Name] = o
	}

	if got := byName["chan"].ChannelTypes; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("expected channel types [0 2] (99 filtered), got %#v", got)
	}
	if len(byName["txt"].ChannelTypes) != 0 {
		t.Errorf("channel types should be ignored on non-channel option, got %#v", byName["txt"].ChannelTypes)
	}

	// round-trip through the DB model and into the discordgo request
	b, _ := json.Marshal(slashCommandData{Description: "d", Options: opts})
	req := buildSlashCommandRequest(&models.CustomCommand{TextTrigger: "t", SlashCommandOptions: null.JSONFrom(b)})
	for _, o := range req.Options {
		if o.Name == "chan" {
			if len(o.ChannelTypes) != 2 || o.ChannelTypes[0] != discordgo.ChannelTypeGuildText || o.ChannelTypes[1] != discordgo.ChannelTypeGuildVoice {
				t.Errorf("channel types not mapped to request: %#v", o.ChannelTypes)
			}
		}
	}
}

func TestBuildSlashCommandRequest(t *testing.T) {
	payload := slashCommandData{
		Description: "manage stuff",
		Options: []SlashCommandOption{
			{Name: "optional_note", Type: int(discordgo.ApplicationCommandOptionString), Description: "a note", Required: false},
			{Name: "user", Type: int(discordgo.ApplicationCommandOptionUser), Description: "target user", Required: true},
		},
	}
	b, _ := json.Marshal(payload)

	cc := &models.CustomCommand{
		TextTrigger:         "manage",
		SlashCommandOptions: null.JSONFrom(b),
	}

	req := buildSlashCommandRequest(cc)

	if req.Name != "manage" {
		t.Errorf("unexpected name %q", req.Name)
	}
	if req.Description != "manage stuff" {
		t.Errorf("unexpected description %q", req.Description)
	}
	if len(req.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(req.Options))
	}
	// required option must be sorted before the optional one
	if req.Options[0].Name != "user" || !req.Options[0].Required {
		t.Errorf("expected required user option first, got %+v", req.Options[0])
	}
	if req.Options[0].Type != discordgo.ApplicationCommandOptionUser {
		t.Errorf("unexpected option type %v", req.Options[0].Type)
	}
}

func TestBuildSlashCommandRequestEmptyDescriptionFallback(t *testing.T) {
	cc := &models.CustomCommand{TextTrigger: "noop"}
	req := buildSlashCommandRequest(cc)
	if req.Description == "" {
		t.Error("expected a fallback description for a command with no stored description")
	}
}

func TestSlashCommandNameRegex(t *testing.T) {
	valid := []string{"foo", "foo-bar", "foo_bar", "abc123", "ünïcode"}
	invalid := []string{"", "Foo Bar", "foo bar", "foo.bar", "thisnameiswaytoolongtobeavalidslashcommandname"}

	for _, v := range valid {
		if !slashCommandNameRegex.MatchString(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	for _, v := range invalid {
		if slashCommandNameRegex.MatchString(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}
