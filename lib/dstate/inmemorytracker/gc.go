package inmemorytracker

import (
	"container/list"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func (shard *ShardTracker) runGcLoop(interval time.Duration) {
	var remainingGuilds []int64

	ticker := time.NewTicker(interval)
	for {
		<-ticker.C
		remainingGuilds = shard.gcTick(time.Now(), remainingGuilds)
	}
}

func (shard *ShardTracker) gcTick(t time.Time, remainingGuilds []int64) []int64 {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if len(remainingGuilds) < 1 {
		remainingGuilds = shard.getGuildIDs()
	}

	for {
		if len(remainingGuilds) < 1 {
			return remainingGuilds
		}

		next := remainingGuilds[0]
		remainingGuilds = remainingGuilds[1:]

		if guild, ok := shard.guilds[next]; ok {
			shard.gcGuild(t, guild)
			break
		}
	}

	return remainingGuilds
}

func (shard *ShardTracker) gcGuild(t time.Time, gs *SparseGuildState) {
	limitLen := shard.conf.ChannelMessageLen
	limitAge := shard.conf.ChannelMessageDur
	if shard.conf.ChannelMessageLimitsF != nil {
		limitLen, limitAge = shard.conf.ChannelMessageLimitsF(gs.Guild.ID)
	}

	if limitLen < 1 && limitAge < 1 {
		return // nothing to do, no limits
	}

	for _, v := range gs.Channels {
		shard.gcGuildChannel(t, gs, v.ID, limitLen, limitAge)
	}

	shard.gcNilMessages(gs)

	if shard.conf.RemoveOfflineMembersAfter > 0 {
		shard.gcMembers(t, gs, shard.conf.RemoveOfflineMembersAfter)
	}
}

func gcGetMessage(messages map[int64]*dstate.MessageState, elem *list.Element) *dstate.MessageState {
	m, _ := messages[elem.Value.(messageView).messageID]
	return m
}

func gcDeleteMessage(messages map[int64]*dstate.MessageState, messagesList *list.List, elem *list.Element) {
	mview := elem.Value.(messageView)
	delete(messages, mview.messageID)
	messagesList.Remove(elem)
}

func (shard *ShardTracker) gcGuildChannel(t time.Time, gs *SparseGuildState, channel int64, maxLen int, maxAge time.Duration) {
	if messages, ok := shard.messages[channel]; ok {
		if messagesView, ok := shard.messageLists[channel]; ok {
			if maxLen > 0 {
				overflow := messagesView.Len() - maxLen
				for i := overflow; i > 0; i-- {
					gcDeleteMessage(messages, messagesView, messagesView.Front())
				}
			}

			if maxAge > 0 {
				if oldest := messagesView.Front(); oldest != nil {
					v := gcGetMessage(messages, oldest)
					age := t.Sub(v.ParsedCreatedAt)

					if age > maxAge {
						shard.gcMessagesAge(t, gs, channel, maxAge, messages, messagesView)
					}
				}
			}
		}
	}
}

func (shard *ShardTracker) gcMessagesAge(t time.Time, gs *SparseGuildState, channel int64, maxAge time.Duration, messages map[int64]*dstate.MessageState, messagesView *list.List) {
	toDel := make([]*list.Element, 0, 100)
	for e := messagesView.Front(); e != nil; e = e.Next() {
		v := gcGetMessage(messages, e)
		if v != nil {
			age := t.Sub(v.ParsedCreatedAt)

			if age > maxAge {
				toDel = append(toDel, e)
			}
		} else {
			toDel = append(toDel, e)
		}
	}

	for _, v := range toDel {
		gcDeleteMessage(messages, messagesView, v)
	}
}

// Clear any messages that are no longer present after channel gc
func (shard *ShardTracker) gcNilMessages(gs *SparseGuildState) {
	messages := shard.guildMessageLists[gs.Guild.ID]
	for e := messages.Front(); e != nil; {
		mview := e.Value.(messageView)
		remove := false

		if cl, ok := shard.messages[mview.channelID]; ok {
			if _, ok := cl[mview.messageID]; !ok {
				remove = true
			}
		} else {
			remove = true
		}

		tmp := e
		e = e.Next()

		if remove {
			messages.Remove(tmp)
		}
	}
}

func (shard *ShardTracker) getGuildIDs() []int64 {
	result := make([]int64, 0, len(shard.guilds))
	for _, v := range shard.guilds {
		result = append(result, v.Guild.ID)
	}

	return result
}

func (shard *ShardTracker) gcMembers(t time.Time, gs *SparseGuildState, maxAge time.Duration) {
	members, ok := shard.members[gs.Guild.ID]
	if !ok {
		return
	}

	for k, v := range members {
		if v.User.ID == shard.conf.BotMemberID {
			continue
		}

		if t.Sub(v.lastUpdated) < maxAge || (v.Presence != nil && v.Presence.Status != dstate.StatusOffline) {
			continue
		}

		delete(members, k)
	}
}
