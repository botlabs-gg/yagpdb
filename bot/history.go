package bot

import (
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

// GetMessages Gets messages from state if possible, if not then it retrieves from the discord api
func GetMessages(guildID int64, channelID int64, limit int, deleted bool) ([]*dstate.MessageState, error) {
	if limit < 1 {
		return []*dstate.MessageState{}, nil
	}

	msgBuf := State.GetMessages(guildID, channelID, &dstate.MessagesQuery{
		Limit:          limit,
		IncludeDeleted: deleted,
	})

	if len(msgBuf) >= limit {
		// State had all messages
		msgBuf = msgBuf[:limit]
		return msgBuf, nil
	}

	// Not enough messages in state, retrieve them from the api
	// Initialize the before id to the oldest message we have
	var before int64
	if len(msgBuf) > 0 {
		before = msgBuf[len(msgBuf)-1].ID
	}

	// Start fetching from the api
	for len(msgBuf) < limit {
		toFetch := limit - len(msgBuf)
		if toFetch > 100 {
			toFetch = 100
		}
		msgs, err := common.BotSession.ChannelMessages(channelID, toFetch, before, 0, 0)
		if err != nil {
			return nil, err
		}

		logger.WithField("num_msgs", len(msgs)).Info("API history req finished")

		if len(msgs) < 1 { // Nothing more
			break
		}

		// Copy over to buffer
		for _, m := range msgs {
			ms := dstate.MessageStateFromDgo(m)
			// ms := dstate.MessageStateFromMessage(m)
			msgBuf = append(msgBuf, ms)
			// msgBuf[nRemaining-k] = ms
		}

		// Oldest message is last
		before = msgs[len(msgs)-1].ID

		if len(msgs) < toFetch {
			// ran out of messages in the channel
			break
		}
	}

	return msgBuf, nil
}

// type DiscordMessages []*dstate.MessageState

// // Len is the number of elements in the collection.
// func (d DiscordMessages) Len() int { return len(d) }

// // Less reports whether the element with
// // index i should sort before the element with index j.
// func (d DiscordMessages) Less(i, j int) bool {
// 	return d[i].ParsedCreated.Before(d[j].ParsedCreated)
// }

// Swap swaps the elements with indexes i and j.
// func (d DiscordMessages) Swap(i, j int) {
// 	temp := d[i]
// 	d[i] = d[j]
// 	d[j] = temp
// }
