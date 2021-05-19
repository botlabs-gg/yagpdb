package state

import (
	"github.com/jonas747/discordgo"
)

// The state system for yags
// You are safe to read everything returned
// You are NOT safe to modify anything returned, as that can cause race conditions
type StateTracker interface {
	GetGuildSet(guildID int64) *CachedGuildSet
	GetMember(guildID int64, memberID int64) *CachedMember

	// channelID is optional and may be 0 to just return guild permissions
	// Returns false if guild, channel or member was not found
	GetMemberPermissions(guildID int64, channelID int64, memberID int64) (perms int64, ok bool)
	// Returns false if guild or channel was not found
	GetRolePermisisons(guildID int64, channelID int64, memberID int64, roles []int64) (perms int64, ok bool)

	// Extras, mainly for performance (all these could be gotten from the above)
	// If you need more than 1 of those, consider saving a call and using GetGuildSet
	GetGuild(guildID int64) *CachedGuild
	GetChannel(guildID int64, channelID int64) *CachedChannel
	GetRole(guildID int64, roleID int64) *CachedRole
	GetEmoji(guildID int64, emojiID int64) *CachedEmoji
}

// Relatively cheap, less frequently updated things
// thinking: should we keep voice states in here? those are more frequently updated but ehhh should we?
type CachedGuildSet struct {
	Guild       *CachedGuild
	Channels    []*CachedChannel
	Roles       []*CachedRole
	Emojis      []*CachedEmoji
	VoiceStates []*discordgo.VoiceState
}

func (gs *CachedGuildSet) GetMemberPermissions(channelID int64, memberID int64, roles []int64) (perms int64, ok bool) {
	ok = true

	var overwrites []discordgo.PermissionOverwrite

	if channel := gs.GetChannel(channelID); channel != nil {
		overwrites = channel.PermissionOverwrites
	} else if channelID != 0 {
		// we still continue as far as we can with the calculations even though we can't apply channel permissions
		ok = false
	}

	perms = CalculatePermissions(gs.Guild, gs.Roles, overwrites, memberID, roles)
	return perms, ok
}

func (gs *CachedGuildSet) GetChannel(id int64) *CachedChannel {
	for _, v := range gs.Channels {
		if v.ID == id {
			return v
		}
	}

	return nil
}

func (gs *CachedGuildSet) GetRole(id int64) *CachedRole {
	for _, v := range gs.Roles {
		if v.ID == id {
			return v
		}
	}

	return nil
}

func (gs *CachedGuildSet) GetVoiceState(userID int64) *discordgo.VoiceState {
	for _, v := range gs.VoiceStates {
		if v.UserID == userID {
			return v
		}
	}

	return nil
}

func (gs *CachedGuildSet) GetEmoji(id int64) *CachedEmoji {
	for _, v := range gs.Emojis {
		if v.ID == id {
			return v
		}
	}

	return nil
}

type CachedGuild struct {
	ID          int64
	Available   bool
	MemberCount int64
	OwnerID     int64
}

func NewCachedGuild(guild *discordgo.Guild) *CachedGuild {
	if guild.Unavailable {
		return &CachedGuild{
			ID:        guild.ID,
			Available: false,
		}
	}

	roleCop := make([]discordgo.Role, len(guild.Roles))
	for i, v := range guild.Roles {
		roleCop[i] = *v
	}

	return &CachedGuild{
		ID:        guild.ID,
		Available: true,
	}
}

type CachedChannel struct {
	ID      int64
	GuildID int64

	PermissionOverwrites []discordgo.PermissionOverwrite
}

func NewCachedChannel(c *discordgo.Channel) *CachedChannel {
	pos := make([]discordgo.PermissionOverwrite, len(c.PermissionOverwrites))
	for i, v := range c.PermissionOverwrites {
		pos[i] = *v
	}

	return &CachedChannel{
		ID:                   c.ID,
		GuildID:              c.GuildID,
		PermissionOverwrites: pos,
	}
}

type CachedRole struct {
	ID          int64
	Permissions int64
}

func NewCachedRole(r *discordgo.Role) *CachedRole {
	return &CachedRole{
		ID:          r.ID,
		Permissions: int64(r.Permissions),
	}
}

type CachedEmoji struct {
	ID      int64
	GuildID int64
}

func NewCachedEmoji(e *discordgo.Emoji) *CachedEmoji {
	return &CachedEmoji{
		ID: e.ID,
	}
}

// A fully cached member
type CachedMember struct {
	// All the sparse fields are always available
	User    discordgo.User
	GuildID int64
	Roles   []int64
	Nick    string

	// These are not always available and all usages should be checked
	Member   *MemberFields
	Presence *PresenceFields
}

type MemberFields struct {
	JoinedAt discordgo.Timestamp
}

type PresenceFields struct {
	// Acitvity here
}

type CachedMessage struct {
	ID        int64
	GuildID   int64
	ChannelID int64

	Author  discordgo.User
	Member  *discordgo.Member
	Content string

	Embeds       []discordgo.MessageEmbed
	Mentions     []discordgo.User
	MentionRoles []int64
	Attachments  []discordgo.MessageAttachment
}

func NewCachedMessage(m *discordgo.Message) *CachedMessage {
	var embeds []discordgo.MessageEmbed
	if len(m.Embeds) > 0 {
		embeds = make([]discordgo.MessageEmbed, len(m.Embeds))
		for i, v := range m.Embeds {
			embeds[i] = *v
		}
	}

	var mentions []discordgo.User
	if len(m.Mentions) > 0 {
		mentions = make([]discordgo.User, len(m.Mentions))
		for i, v := range m.Mentions {
			mentions[i] = *v
		}
	}

	var attachments []discordgo.MessageAttachment
	if len(m.Attachments) > 0 {
		attachments = make([]discordgo.MessageAttachment, len(m.Attachments))
		for i, v := range m.Attachments {
			attachments[i] = *v
		}
	}

	var author discordgo.User
	if m.Author != nil {
		author = *m.Author
	}

	return &CachedMessage{
		ID:        m.ID,
		GuildID:   m.GuildID,
		ChannelID: m.ChannelID,
		Author:    author,
		Member:    m.Member,

		Embeds:       embeds,
		Mentions:     mentions,
		Attachments:  attachments,
		MentionRoles: m.MentionRoles,
	}
}
