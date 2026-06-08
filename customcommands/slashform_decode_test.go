package customcommands

import (
	"net/url"
	"testing"

	"github.com/gorilla/schema"
)

// Regression test for the parallel-array misalignment bug: gorilla/schema drops empty
// values from []string slices, so the slash options are decoded as an indexed struct
// slice (slash_options.N.<field>) instead. This verifies a non-text row with an empty
// choices field doesn't steal the next row's choices, and empty rows are dropped.
func TestSlashFormIndexedDecodeAlignment(t *testing.T) {
	form := url.Values{
		"type":    {"slash_command"},
		"trigger": {"greet"},

		"slash_command_description": {"say hi"},

		// row 0: Integer (min value, no choices). row 1: Text menu with choices. row 2: empty (dropped).
		"slash_options.0.name":      {"amount"},
		"slash_options.0.type":      {"integer"},
		"slash_options.0.choices":   {""},
		"slash_options.0.min_value": {"1"},
		"slash_options.1.name":      {"color"},
		"slash_options.1.type":      {"string_menu"},
		"slash_options.1.choices":   {"red\ngreen\nblue"},
		"slash_options.2.name":      {""},
		"slash_options.2.type":      {"string"},
		"slash_options.2.choices":   {""},
	}

	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	cc := &CustomCommand{}
	if err := dec.Decode(cc, form); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	opts := cc.SlashCommandOptions()
	byName := map[string]SlashCommandOption{}
	for _, o := range opts {
		byName[o.Name] = o
	}

	if len(opts) != 2 {
		t.Fatalf("expected 2 options (empty row dropped), got %d: %#v", len(opts), opts)
	}
	// choices must stay attached to the Text row, not leak to/from the Number row
	if len(byName["color"].Choices) != 3 {
		t.Fatalf("color choices misaligned/lost: %#v", byName["color"].Choices)
	}
	if len(byName["amount"].Choices) != 0 || byName["amount"].MinValue == nil || *byName["amount"].MinValue != 1 {
		t.Fatalf("amount option misaligned: %#v", byName["amount"])
	}
}
