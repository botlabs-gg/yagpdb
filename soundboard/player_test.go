package soundboard

import (
	"errors"
	"testing"
)

func withTestPlayer(t *testing.T, p *Player) {
	t.Helper()
	playersmu.L.Lock()
	players[p.GuildID] = p
	playersmu.L.Unlock()
	t.Cleanup(func() {
		playersmu.L.Lock()
		delete(players, p.GuildID)
		playersmu.L.Unlock()
	})
}

func TestQueueIsCapped(t *testing.T) {
	p := &Player{GuildID: 1, ChannelID: 10}
	for i := 0; i < maxQueuedSounds; i++ {
		p.queue = append(p.queue, &PlayRequest{GuildID: 1, ChannelID: 10, Sound: i})
	}
	withTestPlayer(t, p)

	queued, err := RequestPlaySound(1, 10, 0, 99)
	if !errors.Is(err, ErrQueueFull) || queued {
		t.Fatalf("expected the full queue to be refused, got queued=%v err=%v", queued, err)
	}
}

func TestRequestsWhileStoppingAreKeptForTheNextPlayer(t *testing.T) {
	p := &Player{GuildID: 2, ChannelID: 20, stop: true}
	withTestPlayer(t, p)

	queued, err := RequestPlaySound(2, 21, 0, 5)
	if err != nil || !queued {
		t.Fatalf("expected the request to be accepted, got queued=%v err=%v", queued, err)
	}

	playersmu.L.Lock()
	defer playersmu.L.Unlock()
	if len(p.queue) != 0 || len(p.afterStop) != 1 || p.afterStop[0].Sound != 5 {
		t.Fatalf("expected the request to wait for the next player, queue=%d afterStop=%d", len(p.queue), len(p.afterStop))
	}
}
