package templates

import (
	"fmt"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

func TestUserRegexLimits(t *testing.T) {
	accepted := []string{
		`(?i)^!(help|info|rank)\s*(\d+)?`,
		strings.Repeat("a", 1000),
		`(.|\n){700,}`,
		`\b(H|h)i{2,1000}\b`,
		`(?i)n[il1]{1,1000}(g{2,1000}|q{1,1000})([e3]{1,1000}r|r)`,
	}
	for _, p := range accepted {
		if err := common.CheckUserRegex(p, common.MaxTriggerRegexProgramSize); err != nil {
			t.Errorf("trigger %q rejected: %v", p, err)
		}
	}

	if err := common.CheckUserRegex(strings.Repeat(`.{0,999}`, 10), common.MaxTriggerRegexProgramSize); err == nil {
		t.Error("expected stacked .{0,999} repeats to be rejected as a trigger")
	}
	if err := common.CheckUserRegex(`(?s).{1,1000}`, common.MaxTemplateRegexProgramSize); err != nil {
		t.Errorf("the .{1,1000} chunking idiom should still work in templates: %v", err)
	}
	if err := common.CheckUserRegex(strings.Repeat(`.{0,999}`, 10), common.MaxTemplateRegexProgramSize); err != nil {
		t.Errorf("a template regex compiling to about 20000 should be accepted: %v", err)
	}
	if err := common.CheckUserRegex(strings.Repeat(`.{0,999}`, 60), common.MaxTemplateRegexProgramSize); err == nil {
		t.Error("expected a huge template regex to be rejected")
	}
	if err := common.CheckUserRegex(`(unclosed`, common.MaxTriggerRegexProgramSize); err == nil {
		t.Error("expected an invalid regex to be rejected")
	}
}

func wordListRegex(words int, seed byte) string {
	parts := make([]string, words)
	for i := range parts {
		parts[i] = fmt.Sprintf("%c%dword", 'a'+seed, i)
	}
	return `(?i)\b(` + strings.Join(parts, "|") + `)\b`
}

func TestTemplateRegexBudget(t *testing.T) {
	c := &Context{}

	big := wordListRegex(13000, 0)
	if _, err := c.compileRegex(big); err != nil {
		t.Fatalf("a 13000 word list should compile in a template: %v", err)
	}
	if _, err := c.compileRegex(big); err != nil {
		t.Fatalf("a cached regex should not count against the budget again: %v", err)
	}
	if _, err := c.compileRegex(wordListRegex(13000, 1)); err != nil {
		t.Fatalf("a second large word list should fit the per execution budget: %v", err)
	}
	if _, err := c.compileRegex(wordListRegex(13000, 2)); err == nil {
		t.Fatal("expected the per execution regex budget to be exceeded")
	}
	if _, err := c.compileRegex(`^small$`); err != nil {
		t.Fatalf("a small regex should still fit after a rejected one: %v", err)
	}
}
