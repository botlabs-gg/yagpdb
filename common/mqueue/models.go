package mqueue

import (
	"gopkg.in/src-d/go-kallax.v1"
)

//go:generate kallax gen

type QueuedElement struct {
	kallax.Model `table:"mqueue" pk:"id,autoincr"`

	ID int64

	// Where this feed originated from, responsible for handling discord specific errors
	Source string
	// Could be stuff like reddit feed element id, youtube feed element id and so on
	SourceID string

	// The actual message as a simple string
	MessageStr string

	// The actual message as an embed
	MessageEmbed string

	// The channel to send the message in
	Channel string

	Processed bool
}
