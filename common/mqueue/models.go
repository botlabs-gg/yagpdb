package mqueue

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-kallax.v1"
	"strconv"
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

type QueuedElementNoKallax struct {
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
}

func NewElemFromKallax(existing *QueuedElement) *QueuedElementNoKallax {
	m := &QueuedElementNoKallax{
		ID:         existing.ID,
		Source:     existing.Source,
		SourceID:   existing.SourceID,
		MessageStr: existing.MessageStr,
	}

	channelParsed, _ := strconv.ParseInt(existing.Channel, 10, 64)
	m.Channel = channelParsed

	if len(existing.MessageEmbed) > 0 {
		var dest *discordgo.MessageEmbed
		err := json.Unmarshal([]byte(existing.MessageEmbed), &dest)
		m.MessageEmbed = dest
		if err != nil {
			logrus.WithError(err).Error("Failed decoding mqueue embed")
		}
	}

	return m
}
