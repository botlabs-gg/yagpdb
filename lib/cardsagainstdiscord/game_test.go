package cardsagainstdiscord

// import (
// 	"testing"
// )

// func TestNextCardCzar(t *testing.T) {
// 	t.Log("Testing TestNextCardCzar")
// 	players := []*Player{
// 		{ID: 1, Playing: true, InGame: true},
// 		{ID: 5, Playing: true, InGame: true},
// 		{ID: 2, Playing: true, InGame: true},
// 	}

// 	current := NextCardCzar(players, 0)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 2 {
// 		t.Error("Got ", current, " exected 2")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 5 {
// 		t.Error("Got ", current, " exected 5")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}
// }

// func TestNextCardCzar2(t *testing.T) {
// 	players := []*Player{
// 		{ID: 5, Playing: true, InGame: true},
// 		{ID: 1, Playing: true, InGame: true},
// 		{ID: 2, Playing: true, InGame: true},
// 	}

// 	current := NextCardCzar(players, 0)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 2 {
// 		t.Error("Got ", current, " exected 2")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 5 {
// 		t.Error("Got ", current, " exected 5")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}
// }

// func TestNextCardCzar3(t *testing.T) {
// 	t.Log("Testing TestNextCardCzar3")
// 	players := []*Player{
// 		{ID: 5, Playing: true, InGame: true},
// 		{ID: 1, Playing: true, InGame: true},
// 		{ID: 2, Playing: true, InGame: true},
// 		{ID: 3, Playing: true, InGame: true},
// 	}

// 	current := NextCardCzar(players, 0)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 2 {
// 		t.Error("Got ", current, " exected 2")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 3 {
// 		t.Error("Got ", current, " exected 3")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 5 {
// 		t.Error("Got ", current, " exected 5")
// 	}

// 	current = NextCardCzar(players, current)
// 	if current != 1 {
// 		t.Error("Got ", current, " exected 1")
// 	}
// }

// func TestDupeResponses(t *testing.T) {
// 	t.Log("Testing TestDupeResponses")
// 	for k, pack := range Packs {
// 		g := &Game{
// 			Packs: []string{k},
// 		}

// 		pickedResponses := make([]ResponseCard, 0, len(pack.Responses))
// 		for i := 0; i < len(pack.Responses); i++ {
// 			card := g.getRandomResponseCard()
// 			if card == BlankCard {
// 				i--
// 				continue
// 			}

// 			for _, v := range pickedResponses {
// 				if v == card {
// 					t.Error(k, ": Got duplicate response: ", v)
// 					break
// 				}
// 			}

// 			pickedResponses = append(pickedResponses, card)
// 		}
// 	}
// }

// func TestDupePrompts(t *testing.T) {
// 	t.Log("Testing TestDupePrompts")
// 	for k, pack := range Packs {
// 		g := &Game{
// 			Packs: []string{k},
// 		}

// 		pickedPrompts := make([]string, 0, len(pack.Prompts))
// 		for i := 0; i < len(pack.Prompts); i++ {
// 			prompt := g.randomPrompt()

// 			for _, v := range pickedPrompts {
// 				if v == prompt.Prompt {
// 					t.Error(k, ": Got duplicate prompt: ", v)
// 					break
// 				}
// 			}

// 			pickedPrompts = append(pickedPrompts, prompt.Prompt)
// 		}
// 	}
// }
