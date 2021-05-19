package state

import "github.com/jonas747/discordgo"

type StateTracker interface {
	GetGuildSet(guildID int64) *CachedGuildSet
	GetMember(guildID int64, memberID int64) *CachedMember

	// channelID is optional and may be 0 to just return guild permissions
	// Returns false if channel or member was not found
	GetMemberPermissions(guildID int64, channelID int64, memberID int64) (perms int64, ok bool)
	// Returns false if channel was not found
	GetRolePermisisons(guildID int64, channelID int64, roles []int64) (perms int64, ok bool)

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

func (gs *CachedGuildSet) GetRolesPermissions(channelID int64, roles []int64) (perms int64, ok bool) {
	return 0, true
}

type CachedGuild struct {
	ID          int64
	Available   bool
	MemberCount int64
}

func NewCachedGuild(guild *discordgo.Guild) *CachedGuild {
	if guild.Unavailable {
		return &CachedGuild{
			ID:        guild.ID,
			Available: false,
		}
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
	return &CachedChannel{
		ID:      c.ID,
		GuildID: c.GuildID,
	}
}

type CachedRole struct {
	ID      int64
	GuildID int64
}

func NewCachedRole(r *discordgo.Role) *CachedRole {
	return &CachedRole{
		ID: r.ID,
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
	ID      int64
	GuildID int64
}

// Similar to member, but is sparse and able to be filled in with just presence updates alone
type SparseCachedMember struct {
	ID      int64
	GuildID int64
}

func NewCachedMember(e *discordgo.Member) *CachedMember {
	return &CachedMember{
		ID:      e.User.ID,
		GuildID: e.GuildID,
	}
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
