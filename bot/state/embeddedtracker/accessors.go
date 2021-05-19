package embeddedtracker

import "github.com/jonas747/yagpdb/bot/state"

var _ state.StateTracker = (*SimpleStateTracker)(nil)

func (tracker *SimpleStateTracker) GetGuildSet(guildID int64) *state.CachedGuildSet {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	set, ok := shard.guilds[guildID]
	if !ok {
		return nil
	}

	return &state.CachedGuildSet{
		Guild:       set.Guild,
		Channels:    set.Channels,
		Roles:       set.Roles,
		Emojis:      set.Emojis,
		VoiceStates: set.VoiceStates,
	}
}

func (tracker *SimpleStateTracker) GetMember(guildID int64, memberID int64) *state.CachedMember {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	return shard.getMemberLocked(guildID, memberID)
}

func (shard *SimpleStateTrackerShard) getMemberLocked(guildID int64, memberID int64) *state.CachedMember {
	if ml, ok := shard.members[guildID]; ok {
		for _, v := range ml {
			if v.ID == memberID {
				return v
			}
		}
	}

	return nil
}

func (tracker *SimpleStateTracker) GetMemberPermissions(guildID int64, channelID int64, memberID int64) (perms int64, ok bool) {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	member := shard.getMemberLocked(guildID, memberID)
	if member == nil {
		return 0, false
	}

	return 0, true
}

func (tracker *SimpleStateTracker) GetRolePermisisons(guildID int64, channelID int64, roles []int64) (perms int64, ok bool) {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	return 0, true
}

func (tracker *SimpleStateTracker) getRolePermisisonsLocked(shard *SimpleStateTrackerShard, guildID int64, channelID int64, roles []int64) (perms int64, ok bool) {
	return 0, true
}

func (tracker *SimpleStateTracker) GetGuild(guildID int64) *state.CachedGuild {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if guild, ok := shard.guilds[guildID]; ok {
		return guild.Guild
	}

	return nil
}

func (tracker *SimpleStateTracker) GetChannel(guildID int64, channelID int64) *state.CachedChannel {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if guild, ok := shard.guilds[guildID]; ok {
		for _, v := range guild.Channels {
			if v.ID == channelID {
				return v
			}
		}
	}

	return nil
}

func (tracker *SimpleStateTracker) GetRole(guildID int64, roleID int64) *state.CachedRole {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if guild, ok := shard.guilds[guildID]; ok {
		for _, v := range guild.Roles {
			if v.ID == roleID {
				return v
			}
		}
	}

	return nil
}

func (tracker *SimpleStateTracker) GetEmoji(guildID int64, emojiID int64) *state.CachedEmoji {
	shard := tracker.getShard(guildID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if guild, ok := shard.guilds[guildID]; ok {
		for _, v := range guild.Emojis {
			if v.ID == emojiID {
				return v
			}
		}
	}

	return nil
}

func (tracker *SimpleStateTracker) getShard(guildID int64) *SimpleStateTrackerShard {
	shardID := int((guildID >> 22) % tracker.totalShards)
	return tracker.shards[shardID]
}
