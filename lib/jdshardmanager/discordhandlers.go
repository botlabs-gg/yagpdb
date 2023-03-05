package dshardmanager

import (
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
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
