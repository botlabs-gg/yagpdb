package automod

import (
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/automod/models"
)

func TestConstructReasonIncludesTriggerDetail(t *testing.T) {
	rule := &ParsedRule{Model: &models.AutomodRule{ID: 7, Name: "No slurs"}}
	part := &ParsedPart{ParentRule: rule, RuleModel: &models.AutomodRuleDatum{RuleID: 7}, Part: &WordListTrigger{Blacklist: true}}

	data := &TriggeredRuleData{CurrentRule: rule, ActivatedTriggers: []*ParsedPart{part}}
	if got, want := data.ConstructReason(false), "Triggered rule: No slurs (`Word denylist`)"; got != want {
		t.Fatalf("without detail: got %q want %q", got, want)
	}

	data.TriggerDetails = map[int64]string{7: "`badword` in list `Slurs`"}
	if got, want := data.ConstructReason(false), "Triggered rule: No slurs (`Word denylist`: `badword` in list `Slurs`)"; got != want {
		t.Fatalf("with detail: got %q want %q", got, want)
	}
}

func TestWordListTriggeredReturnsMatchedWord(t *testing.T) {
	set := newWordSet([]string{"bad", "Worse"})

	if word, ok := wordListTriggered(set, []string{"fine", "WORSE", "bad"}, true); !ok || word != "WORSE" {
		t.Fatalf("denylist: got %q, %v", word, ok)
	}
	if word, ok := wordListTriggered(set, []string{"bad", "other"}, false); !ok || word != "other" {
		t.Fatalf("allowlist: got %q, %v", word, ok)
	}
	if _, ok := wordListTriggered(set, []string{"fine"}, true); ok {
		t.Fatal("denylist should not trigger")
	}
}

func TestReasonCodeTextEscapesAndTruncates(t *testing.T) {
	if got := reasonCodeText("a`b"); got != "a'b" {
		t.Fatalf("backtick: got %q", got)
	}
	long := strings.Repeat("é", maxReasonWordLength+10)
	if got := reasonCodeText(long); got != strings.Repeat("é", maxReasonWordLength)+"..." {
		t.Fatalf("truncate: got %q", got)
	}
}
