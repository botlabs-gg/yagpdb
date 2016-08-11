package common

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"sort"
	"time"
)

// Gets mesasges from state if possible, if not then it retrieves from the discord api
// Puts the messages in the state aswell
func GetMessages(channelID string, limit int) ([]*discordgo.Message, error) {
	if limit < 1 {
		return []*discordgo.Message{}, nil
	}

	// check state
	msgBuf := make([]*discordgo.Message, limit)

	state := BotSession.State
	channel, err := state.Channel(channelID)
	if err != nil {
		return nil, err
	}

	state.RLock()
	n := copy(msgBuf, channel.Messages)
	state.RUnlock()

	// Check if the state was full
	if n >= limit {
		return msgBuf, nil
	}

	// Initialize the before id
	before := ""
	if msgBuf[0] != nil {
		before = msgBuf[0].ID
	}

	// Start fetching from the api
	for n < limit {
		toFetch := limit - n
		if toFetch > 100 {
			toFetch = 100
		}
		msgs, err := BotSession.ChannelMessages(channelID, toFetch, before, "")
		if err != nil {
			return nil, err
		}
		log.Println("API history req finihsed", len(msgs))
		if len(msgs) < 1 {
			break
		}

		for k, m := range msgs {
			msgBuf[n+k] = m
		}

		// Oldest message is last
		before = msgs[len(msgs)-1].ID
		n += len(msgs)
	}

	// Remove nil elements incase history wasn't big enough
	msgBuf = msgBuf[:n]

	// merge the current state with this new one and sort
	state.Lock()
	defer state.Unlock()

OUTER:
	for _, cm := range channel.Messages {
		for k, nm := range msgBuf {
			if cm.ID == nm.ID {
				// Update it incase it was changed
				msgBuf[k] = cm
				continue OUTER
			}
		}

		// New message, add it to the buffer
		msgBuf = append(msgBuf, cm)
	}

	sort.Sort(DiscordMessages(msgBuf))

	// And finally apply it to the state
	if state.MaxMessageCount < len(msgBuf) {
		channel.Messages = msgBuf[len(msgBuf)-state.MaxMessageCount:]
	} else {
		channel.Messages = msgBuf
	}

	// Return at most limit results
	if limit < len(msgBuf) {
		return msgBuf[len(msgBuf)-limit:], nil
	} else {
		return msgBuf, nil
	}
}

type DiscordMessages []*discordgo.Message

// Len is the number of elements in the collection.
func (d DiscordMessages) Len() int { return len(d) }

// Less reports whether the element with
// index i should sort before the element with index j.
func (d DiscordMessages) Less(i, j int) bool {
	tsiRaw := d[i].Timestamp
	tsjRaw := d[j].Timestamp
	tsi, _ := time.Parse("2006-01-02T15:04:05.000000-07:00", tsiRaw)
	tsj, _ := time.Parse("2006-01-02T15:04:05.000000-07:00", tsjRaw)
	return tsi.Before(tsj)
}

// Swap swaps the elements with indexes i and j.
func (d DiscordMessages) Swap(i, j int) {
	temp := d[i]
	d[i] = d[j]
	d[j] = temp
}
