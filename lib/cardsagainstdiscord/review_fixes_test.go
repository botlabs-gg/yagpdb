package cardsagainstdiscord

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLiteralPercentIsNotABlank(t *testing.T) {
	prompt := &PromptCard{Prompt: "I'm 100%%sure about %s"}
	if n := CountPromptBlanks(prompt.Prompt); n != 1 {
		t.Fatalf("expected 1 blank, got %d", n)
	}

	if got := prompt.PlaceHolder(); got != "I'm 100%sure about \\_\\_\\_\\_\\_" {
		t.Fatalf("unexpected placeholder: %q", got)
	}
}

func TestDeckRefillsOnceEmpty(t *testing.T) {
	g := &Game{Packs: []string{"ours"}, CustomPacks: []*CardPack{{Name: "ours", Responses: []ResponseCard{"a", "b"}}}}
	g.loadPackResponses()

	if len(g.getRandomPlayerCards(2)) != 2 || len(g.availableResponses) != 0 {
		t.Fatal("expected the deck to be dealt out")
	}

	if len(g.getRandomPlayerCards(3)) != 3 {
		t.Fatal("expected the deck to be reloaded once empty")
	}

	empty := &Game{Packs: []string{"none"}, CustomPacks: []*CardPack{{Name: "none"}}}
	if len(empty.getRandomPlayerCards(3)) != 0 {
		t.Fatal("expected no cards from packs without responses")
	}
}

func TestEveryPlayerGetsTheirCards(t *testing.T) {
	responses := make([]ResponseCard, 100)
	for i := range responses {
		responses[i] = "card"
	}

	full := &Player{ID: 1, InGame: true, Playing: true, Cards: make([]ResponseCard, 9)}
	low := &Player{ID: 2, InGame: true, Playing: true, Cards: make([]ResponseCard, 3)}
	g := &Game{VoteMode: true, Packs: []string{"ours"}, CustomPacks: []*CardPack{{Name: "ours", Responses: responses}}, Players: []*Player{full, low}}
	g.loadPackResponses()

	g.giveEveryoneCards(2)
	if len(full.Cards) != 10 || len(low.Cards) != 5 {
		t.Fatalf("expected the hand limit to only apply per player, got %d and %d cards", len(full.Cards), len(low.Cards))
	}
}

func TestCreateGameRejectsPromptOnlyPacks(t *testing.T) {
	gm := NewGameManager(&StaticSessionProvider{})
	custom := []*CardPack{{Name: "prompts", Prompts: []*PromptCard{{Prompt: "%s"}}}}

	if _, err := gm.CreateGame(CreateGameOptions{Packs: []string{"prompts"}, CustomPacks: custom}); err != ErrAllPacksPromptOnly {
		t.Fatalf("expected a prompt only error, got %v", err)
	}
}

func TestSplitOptionTextKeepsRunesWhole(t *testing.T) {
	text := strings.Repeat("ж", 150)
	label, description := splitOptionText(text)

	if !utf8.ValidString(label) || !utf8.ValidString(description) || utf8.RuneCountInString(label) != 100 || utf8.RuneCountInString(description) != 50 {
		t.Fatalf("unexpected split: %d and %d runes", utf8.RuneCountInString(label), utf8.RuneCountInString(description))
	}

	if label, description := splitOptionText("short"); label != "short" || description != "" {
		t.Fatal("expected short text to stay in the label")
	}
}

func TestVoteDoesNotWaitOnPlayersWithOnlyTheirOwnCards(t *testing.T) {
	a := &Player{ID: 1, InGame: true, Playing: true}
	b := &Player{ID: 2, InGame: true, Playing: true}
	g := &Game{VoteMode: true, Players: []*Player{a, b}, Responses: []*PickedResonse{{Player: a}}}

	if g.waitingForVote(a) {
		t.Fatal("a can only vote for their own cards, so they shouldn't be waited on")
	}

	if !g.waitingForVote(b) {
		t.Fatal("b can vote for a's cards")
	}
}
