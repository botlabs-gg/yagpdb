package mqueue

import (
	"github.com/jonas747/discordgo"
)

type QueuedElement struct {
	ID int64

	// Where this feed originated from, responsible for handling discord specific errors
	Source string
	// Could be stuff like reddit feed element id, youtube feed element id and so on
	SourceID string

	// The actual message as a simple string
	MessageStr string `json:",omitempty"`

	// The actual message as an embed
	MessageEmbed *discordgo.MessageEmbed `json:",omitempty"`

	// The channel to send the message in
	Channel int64
	Guild   int64
}
