package cardsagainstdiscord

import (
	"testing"
	"time"

	"github.com/vmihailenco/msgpack"
)

func TestPreparePack(t *testing.T) {
	pack := &CardPack{Prompts: []*PromptCard{{Prompt: "%s and %s"}, {Prompt: "No blank here"}}}
	PreparePack(pack)

	if pack.Prompts[0].NumPick != 2 {
		t.Fatalf("expected 2 picks, got %d", pack.Prompts[0].NumPick)
	}

	if pack.Prompts[1].NumPick != 1 || pack.Prompts[1].Prompt != "No blank here %s" {
		t.Fatalf("expected a blank added at the end, got %q with %d picks", pack.Prompts[1].Prompt, pack.Prompts[1].NumPick)
	}
}

func TestResolvePackPrefersCustomPacks(t *testing.T) {
	custom := &CardPack{Name: "main", Responses: []ResponseCard{"custom"}}
	g := &Game{CustomPacks: []*CardPack{custom}}

	if g.resolvePack("main") != custom {
		t.Fatal("expected the custom pack to be used over the built in one")
	}

	if g.resolvePack("family-friendly") != Packs["family-friendly"] {
		t.Fatal("expected built in packs to still resolve")
	}

	if g.resolvePack("does-not-exist") != nil {
		t.Fatal("expected unknown packs to resolve to nil")
	}
}

func TestCustomPacksSurviveSerialization(t *testing.T) {
	custom := &CardPack{Name: "ours", Prompts: []*PromptCard{{Prompt: "Why %s?"}}, Responses: []ResponseCard{"because"}}
	PreparePack(custom)

	g := &Game{Packs: []string{"ours"}, CustomPacks: []*CardPack{custom}, PickResponseTime: time.Second * 45}
	encoded, err := msgpack.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Game
	if err := msgpack.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	decoded.loadPackPrompts()
	decoded.loadPackResponses()

	if len(decoded.availablePrompts) != 1 || decoded.availablePrompts[0].NumPick != 1 || len(decoded.availableResponses) != 1 {
		t.Fatalf("custom pack cards were lost: %d prompts, %d responses", len(decoded.availablePrompts), len(decoded.availableResponses))
	}

	if decoded.pickResponseTime() != time.Second*45 || decoded.pickWinnerTime() != PickWinnerDuration {
		t.Fatalf("unexpected timers after serialization: %s %s", decoded.pickResponseTime(), decoded.pickWinnerTime())
	}
}

func TestCreateGameRejectsUnknownPacks(t *testing.T) {
	gm := NewGameManager(&StaticSessionProvider{})

	_, err := gm.CreateGame(CreateGameOptions{Packs: []string{"not-a-pack"}})
	if _, ok := err.(*ErrUnknownPack); !ok {
		t.Fatalf("expected an unknown pack error, got %v", err)
	}

	_, err = gm.CreateGame(CreateGameOptions{Packs: []string{"20-blanks"}})
	if err != ErrAllPacksResponseOnly {
		t.Fatalf("expected a response only error, got %v", err)
	}
}
