package cardsagainstdiscord

import (
	"testing"
)

func newHandTestGame() (*Game, *Player) {
	player := &Player{ID: 2, InGame: true, Playing: true, Cards: []ResponseCard{"a", "b", BlankCard}}
	g := &Game{
		State:           GameStatePickingResponses,
		Round:           3,
		CurrentCardCzar: 1,
		CurrentPropmpt:  &PromptCard{Prompt: "%s and %s", NumPick: 2},
		Players:         []*Player{{ID: 1, InGame: true, Playing: true}, player},
	}

	return g, player
}

func TestHandUnavailableReason(t *testing.T) {
	g, player := newHandTestGame()

	if reason := g.handUnavailableReason(player); reason != "" {
		t.Fatalf("player should be able to pick, got %q", reason)
	}

	if g.handUnavailableReason(nil) == "" {
		t.Fatal("non players should not be able to pick")
	}

	if g.handUnavailableReason(g.findPlayer(1)) == "" {
		t.Fatal("the card czar should not be able to pick")
	}

	g.VoteMode = true
	if reason := g.handUnavailableReason(g.findPlayer(1)); reason != "" {
		t.Fatalf("in vote mode everyone picks, got %q", reason)
	}

	player.Playing = false
	if g.handUnavailableReason(player) == "" {
		t.Fatal("players that joined mid round should not be able to pick")
	}

	player.Playing = true
	g.State = GameStatePickingWinner
	if g.handUnavailableReason(player) == "" {
		t.Fatal("cards should not be pickable outside of picking responses")
	}
}

func TestPlayerPickedCard(t *testing.T) {
	g, player := newHandTestGame()

	if g.playerPickedCard(player, CardSelectionEmojis[0], nil) {
		t.Fatal("a normal card should not open a modal")
	}

	// picking the same card twice or an unknown card does nothing
	g.playerPickedCard(player, CardSelectionEmojis[0], nil)
	g.playerPickedCard(player, "unknown", nil)
	if len(player.SelectedCards) != 1 || player.SelectedCards[0] != 0 {
		t.Fatalf("expected only the first card picked, got %v", player.SelectedCards)
	}

	_, components := g.handMessage(player)
	if len(components) != 1 {
		t.Fatal("expected the hand menu while cards are left to pick")
	}

	g.playerPickedCard(player, CardSelectionEmojis[1], nil)
	if !player.MadeSelections(g.CurrentPropmpt) {
		t.Fatal("expected the player to be done picking")
	}

	if _, components := g.handMessage(player); components != nil {
		t.Fatal("expected no menu once the player is done picking")
	}

	// no more picks than the prompt needs
	g.playerPickedCard(player, CardSelectionEmojis[2], nil)
	if len(player.SelectedCards) != 2 {
		t.Fatalf("expected 2 picks, got %v", player.SelectedCards)
	}
}

func TestHandMenuIDChangesEveryRound(t *testing.T) {
	g, _ := newHandTestGame()
	id := g.handMenuID()

	g.Round++
	if g.handMenuID() == id {
		t.Fatal("menus from previous rounds should have a different id")
	}
}
