package bot

import (
	"sync"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

var MessageDeleteQueue = &messageDeleteQueue{
	channels: make(map[int64]*messageDeleteQueueChannel),
}

type messageDeleteQueue struct {
	sync.RWMutex
	channels         map[int64]*messageDeleteQueueChannel
	customdeleteFunc func(channel int64, msg []int64) error // for testing
}

func (q *messageDeleteQueue) DeleteMessages(guildID int64, channel int64, ids ...int64) {
	q.Lock()
	if cq, ok := q.channels[channel]; ok {
		cq.Lock()

		if !cq.Exiting {
			for _, id := range ids {
				if !common.ContainsInt64Slice(cq.Processing, id) && !common.ContainsInt64Slice(cq.Queued, id) {
					cq.Queued = append(cq.Queued, id)
				}
			}

			cq.Unlock()
			q.Unlock()
			return
		}
	}

	if guildID != 0 {
		if !BotProbablyHasPermission(guildID, channel, discordgo.PermissionManageMessages) {
			q.Unlock()
			return
		}
	}

	// create a new channel queue
	cq := &messageDeleteQueueChannel{
		Parent:  q,
		Channel: channel,
		Queued:  ids,
	}
	q.channels[channel] = cq
	go cq.run()
	q.Unlock()
}

type messageDeleteQueueChannel struct {
	sync.RWMutex

	Parent *messageDeleteQueue

	Channel int64
	Exiting bool

	Queued     []int64
	Processing []int64
}

func (cq *messageDeleteQueueChannel) run() {
	for {
		cq.Lock()
		cq.Processing = nil

		// nothing more to process
		if len(cq.Queued) < 1 {
			cq.Exiting = true

			// remove from parent tracker
			cq.Unlock()
			cq.Parent.Lock()

			// its possible while we unlocked the cq and locked the manager that another queue was launched on the same channel
			// (since we marked cq.Exiting), therefor only delete it from the parent map if we are still the only queue
			if cq.Parent.channels[cq.Channel] == cq {
				delete(cq.Parent.channels, cq.Channel)
			}

			cq.Parent.Unlock()
			return
		}

		if len(cq.Queued) < 100 {
			cq.Processing = cq.Queued
			cq.Queued = nil
		} else {
			cq.Processing = cq.Queued[:99]
			cq.Queued = cq.Queued[99:]
		}

		cq.Unlock()

		cq.processBatch(cq.Processing)
	}
}

func (cq *messageDeleteQueueChannel) processBatch(ids []int64) {
	var err error
	if cq.Parent.customdeleteFunc != nil {
		err = cq.Parent.customdeleteFunc(cq.Channel, ids)
	} else {
		if len(ids) == 1 {
			err = common.BotSession.ChannelMessageDelete(cq.Channel, ids[0])
			if err != nil && common.IsDiscordErr(err, discordgo.ErrCodeUnknownMessage) {
				err = nil
			}
		} else {
			err = common.BotSession.ChannelMessagesBulkDelete(cq.Channel, ids)
		}
	}

	if err != nil {
		logger.WithError(err).Error("delete queue failed deleting messages")
	}

	logger.Debug("Delete queue: deleted msgs ", ids)
}
