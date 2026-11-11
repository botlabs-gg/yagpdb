package cardsagainstdiscord

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestTextContainer(t *testing.T) {
	container := textContainer(colorWinners, "first", "", "second")

	if container.AccentColor != colorWinners || len(container.Components) != 3 {
		t.Fatalf("expected two texts with a divider between them, got %d components", len(container.Components))
	}

	if _, ok := container.Components[1].(discordgo.Separator); !ok {
		t.Fatal("expected a separator between the sections")
	}
}

func TestWithRowsKeepsTheDisplay(t *testing.T) {
	display := textContainer(0, "menu")
	components := withRows(display, GetCommonCahButtons())

	if len(display.Components) != 1 {
		t.Fatal("adding rows should not change the display kept for removing the buttons later")
	}

	sent := components[0].(discordgo.Container)
	if _, ok := sent.Components[len(sent.Components)-1].(discordgo.ActionsRow); !ok {
		t.Fatal("expected the buttons at the bottom of the container")
	}

	if got := withRows(display, nil); len(got[0].(discordgo.Container).Components) != 1 {
		t.Fatal("expected no separator without rows")
	}
}

func TestMenuDisplayShowsNotices(t *testing.T) {
	display := textContainer(0, "menu")
	g := &Game{lastMenuDisplay: &display}

	if len(g.menuDisplay().Components) != 1 {
		t.Fatal("expected only the menu without notices")
	}

	g.menuNotices = []string{"one joined", "two joined"}
	components := g.menuDisplay().Components
	if len(components) != 3 || components[2].(discordgo.TextDisplay).Content != "one joined\ntwo joined" {
		t.Fatalf("expected the notices below the menu, got %d components", len(components))
	}

	if len(display.Components) != 1 {
		t.Fatal("the kept menu should not change when notices are shown")
	}
}

func TestRemoveNotices(t *testing.T) {
	g := &Game{menuNotices: []string{"<@1> Joined the game! (1/10)", startingNotice + "<t:1:R>", "<@2> Joined the game! (2/10)"}}
	g.removeNotices(startingNotice)

	if len(g.menuNotices) != 2 || g.menuNotices[1] != "<@2> Joined the game! (2/10)" {
		t.Fatalf("expected only the countdown to be removed, got %q", g.menuNotices)
	}
}
