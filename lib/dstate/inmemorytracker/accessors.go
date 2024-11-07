package inmemorytracker

import (
	"container/list"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var _ dstate.StateTracker = (*InMemoryTracker)(nil)

func (tracker *InMemoryTracker) GetGuild(guildID int64) *dstate.GuildSet {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	set, ok := shard.guilds[guildID]
	if !ok {
		return nil
	}

	return &dstate.GuildSet{
		GuildState:  *set.Guild,
		Channels:    set.Channels,
		Roles:       set.Roles,
		Emojis:      set.Emojis,
		Stickers:    set.Stickers,
		VoiceStates: set.VoiceStates,
		Threads:     set.Threads,
	}
}

func (tracker *InMemoryTracker) GetMember(guildID int64, memberID int64) *dstate.MemberState {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	ms := shard.getMemberLocked(guildID, memberID)
	if ms != nil {
		return &ms.MemberState
	}

	return nil
}

func (shard *ShardTracker) getMemberLocked(guildID int64, memberID int64) *WrappedMember {

	if members, ok := shard.members[guildID]; ok {
		if ms, ok := members[memberID]; ok {
			return ms
		}
	}

	return nil
}

func (tracker *InMemoryTracker) GetMemberPermissions(guildID int64, channelID int64, memberID int64) (perms int64, ok bool) {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	member := shard.getMemberLocked(guildID, memberID)
	if member == nil || member.Member == nil {
		return 0, false
	}

	return tracker.getRolePermisisonsLocked(shard, guildID, channelID, memberID, member.Member.Roles)
}

func (tracker *InMemoryTracker) GetRolePermisisons(guildID int64, channelID int64, memberID int64, roles []int64) (perms int64, ok bool) {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	return tracker.getRolePermisisonsLocked(shard, guildID, channelID, memberID, roles)
}

func (tracker *InMemoryTracker) getRolePermisisonsLocked(shard *ShardTracker, guildID int64, channelID int64, memberID int64, roles []int64) (perms int64, ok bool) {
	ok = true

	guild, ok := shard.guilds[guildID]
	if !ok {
		return 0, false
	}

	var overwrites []discordgo.PermissionOverwrite

	if channel := guild.channel(channelID); channel != nil {
		overwrites = channel.PermissionOverwrites
	} else if channelID != 0 {
		// we still continue as far as we can with the calculations even though we can't apply channel permissions
		ok = false
	}

	perms = dstate.CalculatePermissions(guild.Guild, guild.Roles, overwrites, memberID, roles)
	return perms, ok
}

func (tracker *InMemoryTracker) getGuildShard(guildID int64) *ShardTracker {
	shardID := int((guildID >> 22) % tracker.totalShards)
	return tracker.shards[shardID]
}

func (tracker *InMemoryTracker) getShard(shardID int64) *ShardTracker {
	return tracker.shards[shardID]
}

func (tracker *InMemoryTracker) cloneMembers(guildID int64) []*dstate.MemberState {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	members, ok := shard.members[guildID]
	if !ok {
		return nil
	}

	membersCop := make([]*dstate.MemberState, 0, len(members))
	if cap(membersCop) < 1 {
		return nil
	}

	for _, v := range members {
		membersCop = append(membersCop, &v.MemberState)
	}

	return membersCop
}

// this IterateMembers implementation is very simple, it makes a full copy of the member slice and calls f in one chunk
func (tracker *InMemoryTracker) IterateMembers(guildID int64, f func(chunk []*dstate.MemberState) bool) {
	members := tracker.cloneMembers(guildID)
	if len(members) < 1 {
		return // nothing to do
	}

	f(members)
}

func (tracker *InMemoryTracker) GetMessages(guildID int64, channelID int64, query *dstate.MessagesQuery) []*dstate.MessageState {
	shard := tracker.getGuildShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	var messages *list.List
	var convert func(*list.Element) *dstate.MessageState

	if channelID == 0 {
		messages = shard.guildMessages[guildID]
		convert = func(e *list.Element) *dstate.MessageState {
			return (*e.Value.(*any)).(*dstate.MessageState)
		}
	} else {
		messages = shard.channelMessages[channelID]
		convert = func(e *list.Element) *dstate.MessageState {
			return e.Value.(*dstate.MessageState)
		}
	}

	if messages == nil {
		return nil
	}

	limit := query.Limit
	if limit < 1 {
		limit = messages.Len()
	}

	buf := query.Buf
	if buf != nil && cap(buf) >= limit {
		buf = buf[:limit]
	} else {
		buf = make([]*dstate.MessageState, limit)
	}

	i := 0
	for e := messages.Back(); e != nil; e = e.Prev() {
		cast := convert(e)
		include, cont := checkMessage(query, cast)
		if include {
			buf[i] = cast
			i++

			if i >= limit {
				break
			}
		}

		if !cont {
			break
		}
	}

	return buf[:i]
}

func checkMessage(q *dstate.MessagesQuery, m *dstate.MessageState) (include bool, continueIter bool) {
	if q.Before != 0 && m.ID >= q.Before {
		return false, true
	}

	if q.After != 0 && m.ID <= q.After {
		return false, false
	}

	if !q.IncludeDeleted && m.Deleted {
		return false, true
	}

	return true, true
}

func (tracker *InMemoryTracker) GetShardGuilds(shardID int64) []*dstate.GuildSet {
	shard := tracker.getShard(shardID)
	if shard == nil {
		return nil
	}

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	gCop := make([]*dstate.GuildSet, 0, len(shard.guilds))
	for _, v := range shard.guilds {
		gCop = append(gCop, &dstate.GuildSet{
			GuildState:  *v.Guild,
			Channels:    v.Channels,
			Roles:       v.Roles,
			Emojis:      v.Emojis,
			Stickers:    v.Stickers,
			VoiceStates: v.VoiceStates,
			Threads:     v.Threads,
		})
	}

	return gCop
}

// SetGuild allows you to manually add guilds to the state tracker, for example when recovering state
func (tracker *InMemoryTracker) SetGuild(gs *dstate.GuildSet) {
	shard := tracker.getGuildShard(gs.ID)
	if shard == nil {
		panic("unknown shard")
	}

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.guilds[gs.ID] = SparseGuildStateFromDstate(gs)

}

// SetMember allows you to manually add members to the state tracker, for example for caching reasons
func (tracker *InMemoryTracker) SetMember(ms *dstate.MemberState) {
	shard := tracker.getGuildShard(ms.GuildID)
	if shard == nil {
		panic("unknown shard")
	}

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.innerHandleMemberUpdate(ms, false)
}

// DelShard allows you to manually reset shards in the state
// notice how i said reset and not delete, as the shards themselves are fixed.
func (tracker *InMemoryTracker) DelShard(shardID int64) {
	shard := tracker.getShard(shardID)
	if shard == nil {
		panic("unknown shard")
	}

	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.reset()
}
