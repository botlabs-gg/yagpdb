package cardsagainstdiscord

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/sirupsen/logrus"
)

const maxMenuNotices = 5

// splitOptionText splits text over the label and the description of a select menu option, which both fit 100 characters
func splitOptionText(text string) (label, description string) {
	runes := []rune(text)
	if len(runes) <= 100 {
		return text, ""
	}

	label, rest := string(runes[:100]), runes[100:]
	if len(rest) > 100 {
		rest = rest[:100]
	}

	return label, string(rest)
}

// waitingForVote returns whether the game still waits on the player's vote, players can't vote for their own cards
// so they're not waited on when theirs are the only ones
func (g *Game) waitingForVote(p *Player) bool {
	if !p.PlayingThisRound() || p.VotedFor != 0 {
		return false
	}

	return slices.ContainsFunc(g.Responses, func(r *PickedResonse) bool { return r.Player.ID != p.ID })
}

// relativeTime shows the time as a timestamp discord keeps counting down, like "in 15 seconds"
func relativeTime(t time.Time) string {
	return fmt.Sprintf("<t:%d:R>", t.Unix())
}

const (
	colorRoundStarted = 7001855
	colorPickWinner   = 5659830
	colorWinners      = 15276265
)

// textContainer is a components v2 container with a text display for each non empty section, divided from each other
func textContainer(color int, sections ...string) discordgo.Container {
	container := discordgo.Container{AccentColor: color}
	for _, v := range sections {
		if v == "" {
			continue
		}

		if len(container.Components) > 0 {
			container.Components = append(container.Components, discordgo.Separator{Divider: true})
		}
		container.Components = append(container.Components, discordgo.TextDisplay{Content: v})
	}

	return container
}

// withRows returns the container with the action rows added at the bottom
func withRows(container discordgo.Container, rows []discordgo.TopLevelComponent) []discordgo.TopLevelComponent {
	if len(rows) > 0 {
		container.Components = append(slices.Clone(container.Components), discordgo.Separator{})
		container.Components = append(container.Components, rows...)
	}

	return []discordgo.TopLevelComponent{container}
}

func (g *Game) sendContainer(container discordgo.Container, rows ...discordgo.TopLevelComponent) (*discordgo.Message, error) {
	return g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Components: withRows(container, rows),
		Flags:      discordgo.MessageFlagsIsComponentsV2,
	})
}

// sendMenu sends a message with the game buttons and removes them from the previous menu
func (g *Game) sendMenu(container discordgo.Container, rows ...discordgo.TopLevelComponent) error {
	m, err := g.sendContainer(container, rows...)
	if err != nil {
		logrus.WithError(err).WithField("channel", g.MasterChannel).Error("Failed sending CAH menu")
		return err
	}

	g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)

	g.LastMenuMessage = m.ID
	g.lastMenuDisplay = &container
	g.lastMenuRows = rows
	g.menuNotices = nil
	return nil
}

// removeNotices removes the announcements starting with the prefix from the last menu, assumes the game is locked
func (g *Game) removeNotices(prefix string) {
	g.menuNotices = slices.DeleteFunc(g.menuNotices, func(n string) bool { return strings.HasPrefix(n, prefix) })
}

// menuDisplay is the last menu along with its announcements, without its buttons
func (g *Game) menuDisplay() discordgo.Container {
	container := *g.lastMenuDisplay
	if len(g.menuNotices) > 0 {
		container.Components = append(slices.Clone(container.Components),
			discordgo.Separator{Divider: true},
			discordgo.TextDisplay{Content: strings.Join(g.menuNotices, "\n")},
		)
	}

	return container
}

// editMenu redraws the last menu in the background, assumes the game is locked
func (g *Game) editMenu() {
	components := withRows(g.menuDisplay(), g.lastMenuRows)
	channelID, messageID := g.MasterChannel, g.LastMenuMessage
	version := g.menuVersion.Add(1)

	go func() {
		g.menuEditMu.Lock()
		defer g.menuEditMu.Unlock()

		if g.menuVersion.Load() != version {
			// a newer edit or menu replaced this one
			return
		}

		g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    channelID,
			ID:         messageID,
			Components: components,
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		})
	}()
}

// removeOldInteractions removes the buttons from the menu message, components v2 messages can't be left without components
// so the menu is redrawn without its buttons
func (g *Game) removeOldInteractions(cID, mID int64) {
	if mID == 0 {
		return
	}

	// edits of the menu that haven't been sent yet would bring its buttons back
	g.menuVersion.Add(1)
	g.menuEditMu.Lock()
	defer g.menuEditMu.Unlock()

	if mID == g.LastMenuMessage && g.lastMenuDisplay != nil {
		g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    cID,
			ID:         mID,
			Components: []discordgo.TopLevelComponent{g.menuDisplay()},
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		})
		return
	}

	// the display isn't kept when a game is restored, and menus from before components v2 were embeds
	_, err := g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    cID,
		ID:         mID,
		Components: []discordgo.TopLevelComponent{},
	})
	if err != nil {
		g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    cID,
			ID:         mID,
			Components: []discordgo.TopLevelComponent{textContainer(0, "-# This game menu is no longer active.")},
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		})
	}
}
