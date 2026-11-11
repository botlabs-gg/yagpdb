package cah

import (
	"html/template"
	"slices"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/cah/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/web"
)

func TestPromptFromLine(t *testing.T) {
	cases := map[string]string{
		"Why ___ and ______?": "Why %s and %s?",
		"100% ___":            "100%% %s",
		"no blanks __ here":   "no blanks __ here",
	}

	for in, expected := range cases {
		if got := PromptFromLine(in); got != expected {
			t.Errorf("%q: expected %q, got %q", in, expected, got)
		}
	}

	if CountBlanks("___ and ___ and ___") != 3 {
		t.Error("expected 3 blanks")
	}
}

func TestParseCardLines(t *testing.T) {
	cards := ParseCardLines("  one card  \r\n\n two words \n")
	if !slices.Equal(cards, []string{"one card", "two words"}) {
		t.Fatalf("unexpected cards: %q", cards)
	}
}

func TestCardPackFromModel(t *testing.T) {
	pack := CardPackFromModel(&models.CahCustomPack{
		Name:      "ours",
		Prompts:   []string{"___ and ___", "No blank"},
		Responses: []string{"a cat"},
	})

	if pack.Prompts[0].NumPick != 2 || pack.Prompts[1].NumPick != 1 {
		t.Fatalf("unexpected picks: %d %d", pack.Prompts[0].NumPick, pack.Prompts[1].NumPick)
	}

	if got := pack.Prompts[0].WithCards([]string{"x", "y"}); got != "**x** and **y**" {
		t.Fatalf("unexpected filled prompt: %q", got)
	}
}

func TestPackFormValidate(t *testing.T) {
	valid := PackForm{Name: " ours ", Prompts: "Why ___?", Responses: "Because"}
	if !valid.Validate(web.TemplateData{}, 1) || valid.Name != "ours" {
		t.Fatalf("expected a valid pack with a trimmed name, got %q", valid.Name)
	}

	invalid := []PackForm{
		{Name: "", Prompts: "Why ___?"},
		{Name: "two words", Prompts: "Why ___?"},
		{Name: "main", Prompts: "Why ___?"},
		{Name: "empty"},
		{Name: "blanks", Prompts: "___ ___ ___ ___"},
		{Name: "long", Responses: strings.Repeat("a", MaxResponseLength+1)},
		{Name: "big", Responses: strings.Repeat("card\n", MaxCardsPerPack+1)},
	}

	for _, v := range invalid {
		if v.Validate(web.TemplateData{}, 1) {
			t.Errorf("expected pack %q to be invalid", v.Name)
		}
	}
}

func TestResolveGamePacks(t *testing.T) {
	conf := DefaultConfig(1)
	conf.DisabledPacks = []string{"redbox"}
	conf.DefaultPacks = []string{"main", "gone"}
	custom := []*cardsagainstdiscord.CardPack{{Name: "ours"}}

	packs, msg := resolveGamePacks(conf, custom, nil)
	if msg != "" || !slices.Equal(packs, []string{"main"}) {
		t.Fatalf("expected the available defaults, got %v %q", packs, msg)
	}

	if _, msg := resolveGamePacks(conf, custom, []string{"redbox"}); msg == "" {
		t.Fatal("expected a turned off pack to be refused")
	}

	packs, _ = resolveGamePacks(conf, custom, []string{"*"})
	if slices.Contains(packs, "redbox") || !slices.Contains(packs, "ours") || !slices.Contains(packs, "main") {
		t.Fatalf("unexpected packs for *: %v", packs)
	}

	conf.DefaultPacks = []string{"gone"}
	if packs, _ := resolveGamePacks(conf, nil, nil); !slices.Equal(packs, []string{"main"}) {
		t.Fatalf("expected main when no defaults are available, got %v", packs)
	}
}

func TestPageTemplateRenders(t *testing.T) {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"checkbox":                func(...any) template.HTML { return "" },
		"textChannelOptionsMulti": func(...any) template.HTML { return "" },
		"roleOptionsMulti":        func(...any) template.HTML { return "" },
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any)
			for i := 0; i+1 < len(values); i += 2 {
				m[values[i].(string)] = values[i+1]
			}
			return m
		},
	}).Parse(`{{define "cp_head"}}{{end}}{{define "cp_alerts"}}{{end}}{{define "cp_footer"}}{{end}}{{define "cp_premium_nudge"}}{{end}}`))
	template.Must(tmpl.Parse(PageHTML))

	prompts, responses := builtinCards(cardsagainstdiscord.Packs["main"])
	builtin := []*BuiltinPackView{{Name: "main", Enabled: true, IsDefault: true, Prompts: prompts, Responses: responses}}

	for _, open := range []bool{false, true} {
		for _, premium := range []bool{true, false} {
			data := map[string]any{
				"ActiveGuild":     map[string]any{"ID": int64(1), "Channels": nil, "Roles": nil},
				"IsGuildPremium":  premium,
				"CAHConfig":       DefaultConfig(1),
				"BuiltinPacks":    builtin,
				"CustomPacks":     []*PackView{{ID: 2, Name: "ours", SavedName: "ours", Prompts: "Why ___?", NumCards: 1, Open: open}},
				"NewPack":         &PackForm{},
				"MaxCustomPacks":  MaxCustomPacks,
				"MaxCardsPerPack": MaxCardsPerPack,
				"MaxPromptBlanks": MaxPromptBlanks,
			}

			var out strings.Builder
			if err := tmpl.ExecuteTemplate(&out, "cp_cah", data); err != nil {
				t.Fatalf("premium %t, pack open %t: %v", premium, open, err)
			}

			if open != strings.Contains(out.String(), `class="collapse show"`) {
				t.Fatalf("expected the custom pack to be expanded only when open, open %t", open)
			}
		}
	}
}

func TestBuiltinCards(t *testing.T) {
	if got := LineFromPrompt("100%% %s and %s"); got != "100% ___ and ___" {
		t.Fatalf("unexpected prompt line: %q", got)
	}

	prompts, responses := builtinCards(cardsagainstdiscord.Packs["20-blanks"])
	if prompts != "" || !strings.HasPrefix(responses, string(cardsagainstdiscord.BlankCard)) {
		t.Fatalf("expected blank cards to be shown as blanks, got %q", common.CutStringShort(responses, 60))
	}

	prompts, _ = builtinCards(cardsagainstdiscord.Packs["main"])
	if strings.Contains(prompts, "%s") || len(strings.Split(prompts, "\n")) != len(cardsagainstdiscord.Packs["main"].Prompts) {
		t.Fatal("expected one line per prompt with blanks shown as ___")
	}
}

func TestPackNames(t *testing.T) {
	for _, name := range []string{"ours", "F.R.I.E.N.D.S.", "my_pack-2", "пакет"} {
		if !packNameRe.MatchString(name) {
			t.Errorf("expected %q to be a valid name", name)
		}
	}

	for _, name := range []string{"two words", "tick`s", "new\nline", "*", "<@&1>"} {
		if packNameRe.MatchString(name) {
			t.Errorf("expected %q to be refused", name)
		}
	}
}

func TestPackFormKeepsDraftOnTooLong(t *testing.T) {
	tmpl := web.TemplateData{}
	form := &PackForm{Name: strings.Repeat("a", maxPackNameLength+1), Prompts: "Why ___?"}

	if form.Validate(tmpl, 1) || tmpl["PackDraft"] != form {
		t.Fatal("expected a too long name to fail and keep what was typed")
	}
}

func TestDefaultFallbackSkipsTurnedOffMain(t *testing.T) {
	conf := DefaultConfig(1)
	conf.DefaultPacks = []string{"gone"}
	conf.DisabledPacks = []string{"main"}

	if packs, _ := resolveGamePacks(conf, nil, nil); len(packs) != 0 {
		t.Fatalf("expected no fallback to a turned off main, got %v", packs)
	}
}

func TestLineFromPromptKeepsLiteralPercent(t *testing.T) {
	if got := LineFromPrompt(PromptFromLine("100%sure ___")); got != "100%sure ___" {
		t.Fatalf("expected the line back as written, got %q", got)
	}
}
