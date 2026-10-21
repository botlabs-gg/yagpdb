package automod

import (
	"reflect"
	"testing"
)

func TestStarterRulePartsMatchTheirTypes(t *testing.T) {
	protections := StarterProtections{ServerInvites: true, MassMentions: true, Spam: true, PhishingLinks: true, AlertChannel: 1}

	rules := protections.rules()
	if len(rules) != 4 {
		t.Fatalf("got %d rules, want 4", len(rules))
	}

	for _, rule := range rules {
		if RulePartMap[rule.parts[0].typeID].Kind() != RulePartTrigger {
			t.Errorf("%s: does not start with a trigger", rule.name)
		}

		for _, part := range rule.parts {
			rulePart, ok := RulePartMap[part.typeID]
			if !ok {
				t.Fatalf("%s: unknown part %d", rule.name, part.typeID)
			}

			want := rulePart.DataType()
			if (want == nil) != (part.settings == nil) {
				t.Errorf("%s: part %d settings %T, want %T", rule.name, part.typeID, part.settings, want)
				continue
			}
			if want != nil && reflect.TypeOf(want).Elem() != reflect.TypeOf(part.settings) {
				t.Errorf("%s: part %d settings %T, want %T", rule.name, part.typeID, part.settings, want)
			}
		}
	}
}

func TestStarterRulesOnlyPicked(t *testing.T) {
	if rules := (StarterProtections{Spam: true}).rules(); len(rules) != 1 || len(rules[0].parts) != 3 {
		t.Errorf("got %+v", rules)
	}
	if rules := (StarterProtections{}).rules(); len(rules) != 0 {
		t.Errorf("got %d rules with nothing picked", len(rules))
	}
}
