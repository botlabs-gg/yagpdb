// Merged message sender sends all the messages in a queue, meged togheter at a interval
// To save on messages send in cases where there can potantially be many
// messages sent in a short interval (such as leave/join announcements with purges)

package bot

import (
	"sync"
	"time"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

var (
	// map of channels and their message queue
	mergedQueue     = make(map[int64][]*QueuedMergedMessage)
	mergedQueueLock sync.Mutex
)

type QueuedMergedMessage struct {
	Content         string
	AllowedMentions discordgo.AllowedMentions
}

func QueueMergedMessage(channelID int64, message string, allowedMentions discordgo.AllowedMentions) {
	mergedQueueLock.Lock()
	defer mergedQueueLock.Unlock()

	mergedQueue[channelID] = append(mergedQueue[channelID], &QueuedMergedMessage{Content: message, AllowedMentions: allowedMentions})
}

func mergedMessageSender() {
	for {
		mergedQueueLock.Lock()

		for c, m := range mergedQueue {
			go sendMergedBatch(c, m)
		}
		mergedQueue = make(map[int64][]*QueuedMergedMessage)
		mergedQueueLock.Unlock()

		time.Sleep(time.Second)
	}
}

func sendMergedBatch(channelID int64, messages []*QueuedMergedMessage) {
	out := ""
	mergedAllowedMentions := discordgo.AllowedMentions{}
	for _, v := range messages {
		out += v.Content + "\n"
		mergedAllowedMentions = mergeAllowedMentions(mergedAllowedMentions, v.AllowedMentions)
	}

	// Strip newline
	out = out[:len(out)-1]
	_, err := dcmd.SplitSendMessage(common.BotSession, channelID, out, mergedAllowedMentions)
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions) {
		logger.WithError(err).WithField("message", out).Error("Error sending messages")
	}
}

// mergeAllowedMentions merges 2 discordgo.AllowedMentions definitions into 1
func mergeAllowedMentions(a, b discordgo.AllowedMentions) discordgo.AllowedMentions {
	// merge mention types
OUTER:
	for _, v := range b.Parse {
		for _, av := range a.Parse {
			if v == av {
				continue OUTER
			}
		}

		a.Parse = append(a.Parse, v)
		switch v {
		case discordgo.AllowedMentionTypeUsers:
			a.Users = nil
			b.Users = nil
		case discordgo.AllowedMentionTypeRoles:
			a.Roles = nil
			b.Roles = nil
		}
	}

	hasParseRoles := false
	hasParseUsers := false
	for _, p := range a.Parse {
		switch p {
		case discordgo.AllowedMentionTypeRoles:
			hasParseRoles = true
		case discordgo.AllowedMentionTypeUsers:
			hasParseUsers = true
		}
	}

	// merge mentioned roles
	if !hasParseRoles {
	OUTER2:
		for _, v := range b.Roles {
			for _, av := range a.Roles {
				if v == av {
					continue OUTER2
				}
			}

			a.Roles = append(a.Roles, v)
		}
	}

	// merge mentioned users
	if !hasParseUsers {
	OUTER3:
		for _, v := range b.Users {
			for _, av := range a.Users {
				if v == av {
					continue OUTER3
				}
			}

			a.Users = append(a.Users, v)
		}
	}

	return a
}
