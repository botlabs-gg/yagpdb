// Merged message sender sends all the messages in a queue, meged togheter at a interval
// To save on messages send in cases where there can potantially be many
// messages sent in a short interval (such as leave/join announcements with purges)

package bot

import (
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common"
)

var (
	// map of channels and their message queue
	mergedQueue     = make(map[int64][]string)
	mergedQueueLock sync.Mutex
)

func QueueMergedMessage(channelID int64, message string) {
	mergedQueueLock.Lock()
	defer mergedQueueLock.Unlock()

	if mergedQueue[channelID] == nil {
		mergedQueue[channelID] = []string{message}
	} else {
		mergedQueue[channelID] = append(mergedQueue[channelID], message)
	}
}

func mergedMessageSender() {
	for {
		mergedQueueLock.Lock()

		for c, m := range mergedQueue {
			go sendMergedBatch(c, m)
		}
		mergedQueue = make(map[int64][]string)
		mergedQueueLock.Unlock()

		time.Sleep(time.Second)
	}
}

func sendMergedBatch(channelID int64, messages []string) {
	out := ""
	for _, v := range messages {
		out += v + "\n"
	}

	// Strip newline
	out = out[:len(out)-1]

	_, err := dutil.SplitSendMessage(common.BotSession, channelID, out)
	if err != nil && !common.IsDiscordErr(err, discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions) {
		logger.WithError(err).WithField("message", out).Error("Error sending messages")
	}
}
