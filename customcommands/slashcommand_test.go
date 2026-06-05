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
		SlashOptionNames:        []string{"opt_a", "  ", "opt_b", "opt_c"},
		SlashOptionTypes:        []int{int(discordgo.ApplicationCommandOptionString), 0, int(discordgo.ApplicationCommandOptionUser), int(discordgo.ApplicationCommandOptionInteger)},
		SlashOptionDescriptions: []string{"desc a", "ignored", "desc b", "desc c"},
		SlashOptionRequired:     []bool{false, false, true, false},
	}

	opts := cc.SlashCommandOptions()

	// the blank-named row must be skipped
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
		SlashOptionNames:        []string{"target", "count"},
		SlashOptionTypes:        []int{int(discordgo.ApplicationCommandOptionUser), int(discordgo.ApplicationCommandOptionInteger)},
		SlashOptionDescriptions: []string{"the target", "how many"},
		SlashOptionRequired:     []bool{true, false},
		Responses:               []string{"hello"},
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
