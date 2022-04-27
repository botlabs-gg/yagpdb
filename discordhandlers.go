package dshardmanager

import (
	"github.com/jonas747/discordgo/v2"
)

func (m *Manager) OnDiscordConnected(s *discordgo.Session, evt *discordgo.Connect) {
	m.handleEvent(EventConnected, s.ShardID, "")
}

func (m *Manager) OnDiscordDisconnected(s *discordgo.Session, evt *discordgo.Disconnect) {
	m.handleEvent(EventDisconnected, s.ShardID, "")
}

func (m *Manager) OnDiscordReady(s *discordgo.Session, evt *discordgo.Ready) {
	m.handleEvent(EventReady, s.ShardID, "")
}

func (m *Manager) OnDiscordResumed(s *discordgo.Session, evt *discordgo.Resumed) {
	m.handleEvent(EventResumed, s.ShardID, "")
}
