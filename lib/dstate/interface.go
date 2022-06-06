package dstate

import (
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// The state system for yags
// You are safe to read everything returned
// You are NOT safe to modify anything returned, as that can cause race conditions
type StateTracker interface {
	// GetGuild returns a guild set for the provided guildID, or nil if not found
	GetGuild(guildID int64) *GuildSet

	// GetShardGuilds returns all the guild sets on the shard
	// this will panic if shardID is below 0 or >= total shards
	GetShardGuilds(shardID int64) []*GuildSet

	// GetMember returns a member from state
	// Note that MemberState.Member is nil if only presence data is present, and likewise for MemberState.Presence
	//
	// returns nil if member is not found in the guild's state
	// which does not mean they're not a member, simply that they're not cached
	GetMember(guildID int64, memberID int64) *MemberState

	// GetMessages returns the messages of the channel, up to limit, you may pass in a pre-allocated buffer to save allocations.
	// If cap(buf) is less than the needed then a new one will be created and returned
	// if len(buf) is greater than needed, it will be sliced to the proper length
	GetMessages(guildID int64, channelID int64, query *MessagesQuery) []*MessageState

	// Calls f on all members, return true to continue or false to stop
	//
	// This is a blocking, non-concurrent operation that returns when f has either returned false or f has been called on all members
	// it should be safe to modify local caller variables within f without needing any syncronization on the caller side
	// as syncronization is done by the implmentation to ensure f is never called concurrently
	//
	// It's up to the implementation to decide how to chunk the results, it may even just be 1 chunk
	// The reason it can be chunked is in the cases where state is remote
	//
	// Note that f may not be called if there were no results
	IterateMembers(guildID int64, f func(chunk []*MemberState) bool)
}

// Relatively cheap, less frequently updated things
// thinking: should we keep voice states in here? those are more frequently updated but ehhh should we?
type GuildSet struct {
	GuildState

	Channels    []ChannelState
	Threads     []ChannelState
	Roles       []discordgo.Role
	Emojis      []discordgo.Emoji
	VoiceStates []discordgo.VoiceState
}

func (gs *GuildSet) GetMemberPermissions(channelID int64, memberID int64, roles []int64) (perms int64, err error) {

	channel := gs.GetChannelOrThread(channelID)
	if channel != nil {
		if channel.Type.IsThread() {
			// use thread parent channel for perms
			channel = gs.GetChannel(channel.ParentID)
		}
	}

	var overwrites []discordgo.PermissionOverwrite
	if channel == nil && channelID != 0 {
		err = &ErrChannelNotFound{
			ChannelID: channelID,
		}
	} else if channel != nil {
		overwrites = channel.PermissionOverwrites
	}

	perms = CalculatePermissions(&gs.GuildState, gs.Roles, overwrites, memberID, roles)
	return perms, err
}

func (gs *GuildSet) GetChannel(id int64) *ChannelState {
	for i := range gs.Channels {
		if gs.Channels[i].ID == id {
			return &gs.Channels[i]
		}
	}

	return nil
}

func (gs *GuildSet) GetRole(id int64) *discordgo.Role {
	for i := range gs.Roles {
		if gs.Roles[i].ID == id {
			return &gs.Roles[i]
		}
	}

	return nil
}

func (gs *GuildSet) GetVoiceState(userID int64) *discordgo.VoiceState {
	for i := range gs.VoiceStates {
		if gs.VoiceStates[i].UserID == userID {
			return &gs.VoiceStates[i]
		}
	}

	return nil
}

func (gs *GuildSet) GetEmoji(id int64) *discordgo.Emoji {
	for i := range gs.Emojis {
		if gs.Emojis[i].ID == id {
			return &gs.Emojis[i]
		}
	}

	return nil
}

func (gs *GuildSet) GetThread(id int64) *ChannelState {
	for i := range gs.Threads {
		if gs.Threads[i].ID == id {
			return &gs.Threads[i]
		}
	}

	return nil
}

func (gs *GuildSet) GetChannelOrThread(id int64) *ChannelState {
	if cs := gs.GetChannel(id); cs != nil {
		return cs
	}

	return gs.GetThread(id)
}

type GuildState struct {
	ID          int64  `json:"id,string"`
	Available   bool   `json:"available"`
	MemberCount int64  `json:"member_count"`
	OwnerID     int64  `json:"owner_id,string"`
	Region      string `json:"region"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`

	Description string `json:"description"`

	PreferredLocale string `json:"preferred_locale"`

	// The ID of the AFK voice channel.
	AfkChannelID int64 `json:"afk_channel_id,string"`

	// The hash of the guild's splash.
	Splash string `json:"splash"`

	// The timeout, in seconds, before a user is considered AFK in voice.
	AfkTimeout int `json:"afk_timeout"`

	// The verification level required for the guild.
	VerificationLevel discordgo.VerificationLevel `json:"verification_level"`

	// Whether the guild is considered large. This is
	// determined by a member threshold in the identify packet,
	// and is currently hard-coded at 250 members in the library.
	Large bool `json:"large"`

	// The default message notification setting for the guild.
	// 0 == all messages, 1 == mentions only.
	DefaultMessageNotifications int `json:"default_message_notifications"`

	MaxPresences int `json:"max_presences"`
	MaxMembers   int `json:"max_members"`

	// Whether this guild is currently unavailable (most likely due to outage).
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	Unavailable bool `json:"unavailable"`

	// The explicit content filter level
	ExplicitContentFilter discordgo.ExplicitContentFilterLevel `json:"explicit_content_filter"`

	// The list of enabled guild features
	Features []string `json:"features"`

	// Required MFA level for the guild
	MfaLevel discordgo.MfaLevel `json:"mfa_level"`

	// Whether or not the Server Widget is enabled
	WidgetEnabled bool `json:"widget_enabled"`

	// The Channel ID for the Server Widget
	WidgetChannelID string `json:"widget_channel_id"`

	// The Channel ID to which system messages are sent (eg join and leave messages)
	SystemChannelID string `json:"system_channel_id"`
}

type ChannelState struct {
	ID               int64                     `json:"id,string"`
	GuildID          int64                     `json:"guild_id,string"`
	Name             string                    `json:"name"`
	Topic            string                    `json:"topic"`
	Type             discordgo.ChannelType     `json:"type"`
	NSFW             bool                      `json:"nsfw"`
	Icon             string                    `json:"icon"`
	Position         int                       `json:"position"`
	Bitrate          int                       `json:"bitrate"`
	UserLimit        int                       `json:"user_limit"`
	ParentID         int64                     `json:"parent_id,string"`
	RateLimitPerUser int                       `json:"rate_limit_per_user"`
	OwnerID          int64                     `json:"owner_id,string"`
	ThreadMetadata   *discordgo.ThreadMetadata `json:"thread_metadata"`

	PermissionOverwrites []discordgo.PermissionOverwrite `json:"permission_overwrites"`
}

func (c *ChannelState) IsPrivate() bool {
	if c.Type == discordgo.ChannelTypeDM || c.Type == discordgo.ChannelTypeGroupDM {
		return true
	}

	return false
}

// A fully cached member
type MemberState struct {
	// All the sparse fields are always available
	User    discordgo.User
	GuildID int64

	// These are not always available and all usages should be checked
	Member   *MemberFields
	Presence *PresenceFields
}

type MemberFields struct {
	JoinedAt         discordgo.Timestamp
	Roles            []int64
	Nick             string
	Avatar           string
	Pending          bool
	TimeoutExpiresAt *time.Time
}

type PresenceStatus int32

const (
	StatusNotSet       PresenceStatus = 0
	StatusOnline       PresenceStatus = 1
	StatusIdle         PresenceStatus = 2
	StatusDoNotDisturb PresenceStatus = 3
	StatusInvisible    PresenceStatus = 4
	StatusOffline      PresenceStatus = 5
)

type PresenceFields struct {
	// Acitvity here
	Game   *LightGame
	Status PresenceStatus
}

type LightGame struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Details string `json:"details,omitempty"`
	State   string `json:"state,omitempty"`

	Type discordgo.GameType `json:"type"`
}

func MemberStateFromMember(member *discordgo.Member) *MemberState {
	var user discordgo.User
	if member.User != nil {
		user = *member.User
	}

	return &MemberState{
		User:    user,
		GuildID: member.GuildID,

		Member: &MemberFields{
			JoinedAt:         member.JoinedAt,
			Roles:            member.Roles,
			Nick:             member.Nick,
			Avatar:           member.Avatar,
			Pending:          member.Pending,
			TimeoutExpiresAt: member.TimeoutExpiresAt,
		},
		Presence: nil,
	}
}

// Converts a member state into a discordgo.Member
// this will return nil if member is not available
func (ms *MemberState) DgoMember() *discordgo.Member {
	if ms.Member == nil {
		return nil
	}

	m := &discordgo.Member{
		GuildID:          ms.GuildID,
		JoinedAt:         ms.Member.JoinedAt,
		Nick:             ms.Member.Nick,
		Avatar:           ms.Member.Avatar,
		Roles:            ms.Member.Roles,
		User:             &ms.User,
		Pending:          ms.Member.Pending,
		TimeoutExpiresAt: ms.Member.TimeoutExpiresAt,
	}

	if ms.Member != nil {
		m.JoinedAt = ms.Member.JoinedAt
	}

	return m
}

type MessageState struct {
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

	ParsedCreatedAt time.Time
	ParsedEditedAt  time.Time

	Deleted bool
}

func (m *MessageState) ContentWithMentionsReplaced() string {
	content := m.Content

	for _, user := range m.Mentions {
		content = strings.NewReplacer(
			"<@"+strconv.FormatInt(user.ID, 10)+">", "@"+user.Username,
			"<@!"+strconv.FormatInt(user.ID, 10)+">", "@"+user.Username,
		).Replace(content)
	}

	return content
}

var _ error = (*ErrGuildNotFound)(nil)

type ErrGuildNotFound struct {
	GuildID int64
}

func (e *ErrGuildNotFound) Error() string {
	return "Guild not found: " + strconv.FormatInt(e.GuildID, 10)
}

var _ error = (*ErrChannelNotFound)(nil)

type ErrChannelNotFound struct {
	ChannelID int64
}

func (e *ErrChannelNotFound) Error() string {
	return "Channel not found: " + strconv.FormatInt(e.ChannelID, 10)
}

// IsGuildNotFound returns true if a ErrGuildNotFound, and also the GuildID if it was
func IsGuildNotFound(e error) (bool, int64) {
	if gn, ok := e.(*ErrGuildNotFound); ok {
		return true, gn.GuildID
	}

	return false, 0
}

// IsChannelNotFound returns true if a ErrChannelNotFound, and also the ChannelID if it was
func IsChannelNotFound(e error) (bool, int64) {
	if cn, ok := e.(*ErrChannelNotFound); ok {
		return true, cn.ChannelID
	}

	return false, 0
}

type MessagesQuery struct {
	Buf []*MessageState

	// Get messages made before this ID (message_id <  before)
	Before int64

	// Get messages made after this ID (message_id > after)
	After int64

	Limit          int
	IncludeDeleted bool
}
