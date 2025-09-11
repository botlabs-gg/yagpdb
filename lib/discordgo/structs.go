// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains all structures for the discordgo package.  These
// may be moved about later into separate files but I find it easier to have
// them all located together.

package discordgo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/gojay"
	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
)

// A Session represents a connection to the Discord API.
type Session struct {
	// General configurable settings.

	// Authentication token for this session
	Token   string
	MFA     bool
	Intents []GatewayIntent

	// Debug for printing JSON request/responses
	Debug    bool // Deprecated, will be removed.
	LogLevel int

	// Should the session reconnect the websocket on errors.
	ShouldReconnectOnError bool

	// Should the session request compressed websocket data.
	Compress bool

	// Sharding
	ShardID    int
	ShardCount int

	// Should state tracking be enabled.
	// State tracking is the best way for getting the the users
	// active guilds and the members of the guilds.
	StateEnabled bool

	// Whether or not to call event handlers synchronously.
	// e.g false = launch event handlers in their own goroutines.
	SyncEvents bool

	// Max number of REST API retries
	MaxRestRetries int

	// Managed state object, updated internally with events when
	// StateEnabled is true.
	State *State

	// The http client used for REST requests
	Client *http.Client

	// Stores the last HeartbeatAck that was recieved (in UTC)
	LastHeartbeatAck time.Time

	// used to deal with rate limits
	Ratelimiter *RateLimiter

	// The gateway websocket connection
	GatewayManager *GatewayConnectionManager

	tokenInvalid *int32

	// Event handlers
	handlersMu   sync.RWMutex
	handlers     map[string][]*eventHandlerInstance
	onceHandlers map[string][]*eventHandlerInstance
}

// UserConnection is a Connection returned from the UserConnections endpoint
type UserConnection struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Revoked      bool           `json:"revoked"`
	Integrations []*Integration `json:"integrations"`
}

// Integration stores integration information
type Integration struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Type              string             `json:"type"`
	Enabled           bool               `json:"enabled"`
	Syncing           bool               `json:"syncing"`
	RoleID            string             `json:"role_id"`
	ExpireBehavior    int                `json:"expire_behavior"`
	ExpireGracePeriod int                `json:"expire_grace_period"`
	User              *User              `json:"user"`
	Account           IntegrationAccount `json:"account"`
	SyncedAt          Timestamp          `json:"synced_at"`
}

// IntegrationAccount is integration account information
// sent by the UserConnections endpoint
type IntegrationAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// A VoiceRegion stores data for a specific voice region server.
type VoiceRegion struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"sample_hostname"`
	Port     int    `json:"sample_port"`
}

// A VoiceICE stores data for voice ICE servers.
type VoiceICE struct {
	TTL     string       `json:"ttl"`
	Servers []*ICEServer `json:"servers"`
}

// A ICEServer stores data for a specific voice ICE server.
type ICEServer struct {
	URL        string `json:"url"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

// A Invite stores all data related to a specific Discord Guild or Channel invite.
type Invite struct {
	Guild     *Guild    `json:"guild"`
	Channel   *Channel  `json:"channel"`
	Inviter   *User     `json:"inviter"`
	Code      string    `json:"code"`
	CreatedAt Timestamp `json:"created_at"`
	MaxAge    int       `json:"max_age"`
	Uses      int       `json:"uses"`
	MaxUses   int       `json:"max_uses"`
	Revoked   bool      `json:"revoked"`
	Temporary bool      `json:"temporary"`
	Unique    bool      `json:"unique"`

	// will only be filled when using InviteWithCounts
	ApproximatePresenceCount int `json:"approximate_presence_count"`
	ApproximateMemberCount   int `json:"approximate_member_count"`
}

// ForumSortOrderType represents sort order of a forum channel.
type ForumSortOrderType int

const (
	// ForumSortOrderLatestActivity sorts posts by activity.
	ForumSortOrderLatestActivity ForumSortOrderType = 0
	// ForumSortOrderCreationDate sorts posts by creation time (from most recent to oldest).
	ForumSortOrderCreationDate ForumSortOrderType = 1
)

// ForumLayout represents layout of a forum channel.
type ForumLayout int

const (
	// ForumLayoutNotSet represents no default layout.
	ForumLayoutNotSet ForumLayout = 0
	// ForumLayoutListView displays forum posts as a list.
	ForumLayoutListView ForumLayout = 1
	// ForumLayoutGalleryView displays forum posts as a collection of tiles.
	ForumLayoutGalleryView ForumLayout = 2
)

// the thread will stop showing in the channel list after auto_archive_duration minutes of inactivity, can be set to:
// 60, 1440, 4320, 10080
type AutoArchiveDuration int

const (
	AutoArchiveDurationOneHour   AutoArchiveDuration = 60
	AutoArchiveDurationOneDay    AutoArchiveDuration = 1440
	AutoArchiveDurationThreeDays AutoArchiveDuration = 4320
	AutoArchiveDurationOneWeek   AutoArchiveDuration = 10080
)

// ChannelType is the type of a Channel
type ChannelType int

// Block contains known ChannelType values
const (
	ChannelTypeGuildText          ChannelType = 0  // a text channel within a server
	ChannelTypeDM                 ChannelType = 1  // a direct message between users
	ChannelTypeGuildVoice         ChannelType = 2  // a voice channel within a server
	ChannelTypeGroupDM            ChannelType = 3  // a direct message between multiple users
	ChannelTypeGuildCategory      ChannelType = 4  // an organizational category that contains up to 50 channels
	ChannelTypeGuildNews          ChannelType = 5  // a channel that users can follow and crosspost into their own server
	ChannelTypeGuildStore         ChannelType = 6  // a channel in which game developers can sell their game on Discord
	ChannelTypeGuildNewsThread    ChannelType = 10 // a temporary sub-channel within a GUILD_NEWS channel
	ChannelTypeGuildPublicThread  ChannelType = 11 // a temporary sub-channel within a GUILD_TEXT channel
	ChannelTypeGuildPrivateThread ChannelType = 12 // a temporary sub-channel within a GUILD_TEXT channel that is only viewable by those invited and those with the MANAGE_THREADS permission
	ChannelTypeGuildStageVoice    ChannelType = 13 // a voice channel for hosting events with an audience
	ChannelTypeGuildForum         ChannelType = 15 // a channel that can only contain threads
)

func (t ChannelType) IsThread() bool {
	return t == ChannelTypeGuildPrivateThread || t == ChannelTypeGuildPublicThread || t == ChannelTypeGuildNewsThread
}

// A Channel holds all data related to an individual Discord channel.
type Channel struct {
	// The ID of the channel.
	ID int64 `json:"id,string"`

	// The ID of the guild to which the channel belongs, if it is in a guild.
	// Else, this ID is empty (e.g. DM channels).
	GuildID int64 `json:"guild_id,string"`

	// The name of the channel.
	Name string `json:"name"`

	// The topic of the channel.
	Topic string `json:"topic"`

	// The type of the channel.
	Type ChannelType `json:"type"`

	// The ID of the last message sent in the channel. This is not
	// guaranteed to be an ID of a valid message.
	LastMessageID int64 `json:"last_message_id,string"`

	// Whether the channel is marked as NSFW.
	NSFW bool `json:"nsfw"`

	// Icon of the group DM channel.
	Icon string `json:"icon"`

	// The position of the channel, used for sorting in client.
	Position int `json:"position"`

	// The bitrate of the channel, if it is a voice channel.
	Bitrate int `json:"bitrate"`

	// The recipients of the channel. This is only populated in DM channels.
	Recipients []*User `json:"recipients"`

	// The messages in the channel. This is only present in state-cached channels,
	// and State.MaxMessageCount must be non-zero.
	Messages []*Message `json:"-"`

	// A list of permission overwrites present for the channel.
	PermissionOverwrites []*PermissionOverwrite `json:"permission_overwrites"`

	// The user limit of the voice channel.
	UserLimit int `json:"user_limit"`

	// The ID of the parent channel, if the channel is under a category
	ParentID int64 `json:"parent_id,string"`

	// Period of time in which the user is unable to send another message or create a new thread for.
	// Excludes "manage_messages" and "manage_channel" permissions
	RateLimitPerUser int `json:"rate_limit_per_user"`

	// The user ID of the owner of the thread. Nil on normal channels
	OwnerID int64 `json:"owner_id,string"`

	// Thread specific fields
	ThreadMetadata *ThreadMetadata `json:"thread_metadata"`

	// ChannelFlags combined as a bitfield.
	Flags ChannelFlags `json:"flags"`

	// The set of tags that can be used in a forum channel.
	AvailableTags []ForumTag `json:"available_tags"`

	// The IDs of the set of tags that have been applied to a thread in a forum channel.
	AppliedTags IDSlice `json:"applied_tags"`

	// Emoji to use as the default reaction to a forum post.
	DefaultReactionEmoji ForumDefaultReaction `json:"default_reaction_emoji"`

	// The initial RateLimitPerUser to set on newly created threads in a channel.
	// This field is copied to the thread at creation time and does not live update.
	DefaultThreadRateLimitPerUser int `json:"default_thread_rate_limit_per_user"`

	// The default sort order type used to order posts in forum channels.
	// Defaults to null, which indicates a preferred sort order hasn't been set by a channel admin.
	DefaultSortOrder *ForumSortOrderType `json:"default_sort_order"`

	// The default forum layout view used to display posts in forum channels.
	// Defaults to ForumLayoutNotSet, which indicates a layout view has not been set by a channel admin.
	DefaultForumLayout ForumLayout `json:"default_forum_layout"`
}

func (c *Channel) GetChannelID() int64 {
	return c.ID
}

func (c *Channel) GetGuildID() int64 {
	return c.GuildID
}

// Mention returns a string which mentions the channel
func (c *Channel) Mention() string {
	return fmt.Sprintf("<#%d>", c.ID)
}

// ChannelFlags is the flags of "channel" (see ChannelFlags* consts)
// https://discord.com/developers/docs/resources/channel#message-object-message-flags
type ChannelFlags int

// Valid ChannelFlags values
const (
	// ChannelFlagsPinned this thread is pinned to the top of its parent GUILD_FORUM or GUILD_MEDIA channel.
	ChannelFlagsPinned ChannelFlags = 1 << 1
	// ChannelFlagsRequireTag whether a tag is required to be specified when creating a thread in a GUILD_FORUM or a GUILD_MEDIA channel. Tags are specified in the applied_tags field.
	ChannelFlagsRequireTag ChannelFlags = 1 << 4
	// ChannelFlagsHideMediaDownloadOptions when set hides the embedded media download options. Available only for media channels.
	ChannelFlagsHideMediaDownloadOptions ChannelFlags = 1 << 15
)

// A ChannelEdit holds Channel Field data for a channel edit.
type ChannelEdit struct {
	Name                 string                 `json:"name,omitempty"`
	Topic                string                 `json:"topic,omitempty"`
	NSFW                 bool                   `json:"nsfw,omitempty"`
	Position             *int                   `json:"position,omitempty"`
	Bitrate              int                    `json:"bitrate,omitempty"`
	UserLimit            int                    `json:"user_limit,omitempty"`
	PermissionOverwrites []*PermissionOverwrite `json:"permission_overwrites,omitempty"`
	ParentID             *null.String           `json:"parent_id,omitempty"`
	Flags                *ChannelFlags          `json:"flags,omitempty"`
	RateLimitPerUser     *int                   `json:"rate_limit_per_user,omitempty"`

	// Threads only
	Archived            *bool               `json:"archived,omitempty"`
	AutoArchiveDuration AutoArchiveDuration `json:"auto_archive_duration,omitempty"`
	Locked              *bool               `json:"locked,omitempty"`
	Invitable           *bool               `json:"invitable,omitempty"`

	// NOTE: forum threads only - these are IDs
	AppliedTags IDSlice `json:"applied_tags,string,omitempty"`
}

type RoleCreate struct {
	Name        string `json:"name,omitempty"`
	Permissions int64  `json:"permissions,string,omitempty"`
	Color       int32  `json:"color,omitempty"`
	Hoist       bool   `json:"hoist"`
	Mentionable bool   `json:"mentionable"`
}

// A PermissionOverwrite holds permission overwrite data for a Channel
type PermissionOverwrite struct {
	ID    int64                   `json:"id,string"`
	Type  PermissionOverwriteType `json:"type"`
	Deny  int64                   `json:"deny,string"`
	Allow int64                   `json:"allow,string"`
}

type PermissionOverwriteType int

const (
	PermissionOverwriteTypeRole   PermissionOverwriteType = 0
	PermissionOverwriteTypeMember PermissionOverwriteType = 1
)

// ThreadStart stores all parameters you can use with MessageThreadStartComplex or ThreadStartComplex
type ThreadStart struct {
	Name                string              `json:"name"`
	AutoArchiveDuration AutoArchiveDuration `json:"auto_archive_duration,omitempty"`
	Type                ChannelType         `json:"type,omitempty"`
	Invitable           bool                `json:"invitable"`
	RateLimitPerUser    int                 `json:"rate_limit_per_user,omitempty"`

	// NOTE: forum threads only - these are IDs
	AppliedTags IDSlice `json:"applied_tags,string,omitempty"`
}

// ThreadsList represents a list of threads alongisde with thread member objects for the current user.
type ThreadsList struct {
	Threads []*Channel      `json:"threads"`
	Members []*ThreadMember `json:"members"`
	HasMore bool            `json:"has_more"`
}

// AddedThreadMember holds information about the user who was added to the thread
type AddedThreadMember struct {
	*ThreadMember
	Member   *Member   `json:"member"`
	Presence *Presence `json:"presence"`
}

// ForumDefaultReaction specifies emoji to use as the default reaction to a forum post.
// NOTE: Exactly one of EmojiID and EmojiName must be set.
type ForumDefaultReaction struct {
	// The id of a guild's custom emoji.
	EmojiID int64 `json:"emoji_id,string,omitempty"`
	// The unicode character of the emoji.
	EmojiName string `json:"emoji_name,omitempty"`
}

// ForumTag represents a tag that is able to be applied to a thread in a forum channel.
type ForumTag struct {
	ID        int64  `json:"id,string,omitempty"`
	Name      string `json:"name"`
	Moderated bool   `json:"moderated"`
	EmojiID   int64  `json:"emoji_id,string,omitempty"`
	EmojiName string `json:"emoji_name,omitempty"`
}

// Emoji struct holds data related to Emoji's
type Emoji struct {
	ID            int64   `json:"id,string"`
	Name          string  `json:"name"`
	Roles         IDSlice `json:"roles,string"`
	Managed       bool    `json:"managed"`
	RequireColons bool    `json:"require_colons"`
	Animated      bool    `json:"animated"`
}

// MessageFormat returns a correctly formatted Emoji for use in Message content and embeds
func (e *Emoji) MessageFormat() string {
	if e.ID != 0 && e.Name != "" {
		if e.Animated {
			return "<a:" + e.APIName() + ">"
		}

		return "<:" + e.APIName() + ">"
	}

	return e.APIName()
}

// APIName returns an correctly formatted API name for use in the MessageReactions endpoints.
func (e *Emoji) APIName() string {
	if e.ID != 0 && e.Name != "" {
		return e.Name + ":" + StrID(e.ID)
	}
	if e.Name != "" {
		return e.Name
	}
	return StrID(e.ID)
}

// StickerFormat is the file format of the Sticker.
type StickerFormat int

// Defines all known Sticker types.
const (
	StickerFormatTypePNG    StickerFormat = 1
	StickerFormatTypeAPNG   StickerFormat = 2
	StickerFormatTypeLottie StickerFormat = 3
	StickerFormatTypeGIF    StickerFormat = 4
)

// StickerType is the type of Sticker.
type StickerType int

// Defines Sticker types.
const (
	StickerTypeStandard StickerType = 1
	StickerTypeGuild    StickerType = 2
)

// Sticker represents a Sticker object that can be sent in a Message.
type Sticker struct {
	ID          int64         `json:"id,string"`
	PackID      int64         `json:"pack_id,string"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tags        string        `json:"tags"`
	Type        StickerType   `json:"type"`
	FormatType  StickerFormat `json:"format_type"`
	Available   bool          `json:"available"`
	GuildID     int64         `json:"guild_id,string"`
	User        *User         `json:"user"`
	SortValue   int           `json:"sort_value"`
}

// StickerItem represents the smallest amount of data required to render a sticker. A partial sticker object.
type StickerItem struct {
	ID         int64         `json:"id,string"`
	Name       string        `json:"name"`
	FormatType StickerFormat `json:"format_type"`
}

// StickerPack represents a pack of standard stickers.
type StickerPack struct {
	ID             int64     `json:"id,string"`
	Stickers       []Sticker `json:"stickers"`
	Name           string    `json:"name"`
	SKUID          int64     `json:"sku_id,string"`
	CoverStickerID int64     `json:"cover_sticker_id,string"`
	Description    string    `json:"description"`
	BannerAssetID  int64     `json:"banner_asset_id,string"`
}

// VerificationLevel type definition
type VerificationLevel int

// Constants for VerificationLevel levels from 0 to 3 inclusive
const (
	VerificationLevelNone VerificationLevel = iota
	VerificationLevelLow
	VerificationLevelMedium
	VerificationLevelHigh
)

// ExplicitContentFilterLevel type definition
type ExplicitContentFilterLevel int

// Constants for ExplicitContentFilterLevel levels from 0 to 2 inclusive
const (
	ExplicitContentFilterDisabled ExplicitContentFilterLevel = iota
	ExplicitContentFilterMembersWithoutRoles
	ExplicitContentFilterAllMembers
)

// MfaLevel type definition
type MfaLevel int

// Constants for MfaLevel levels from 0 to 1 inclusive
const (
	MfaLevelNone MfaLevel = iota
	MfaLevelElevated
)

// A Guild holds all data related to a specific Discord Guild.  Guilds are also
// sometimes referred to as Servers in the Discord client.
type Guild struct {
	// The ID of the guild.
	ID int64 `json:"id,string"`

	// The name of the guild. (2–100 characters)
	Name string `json:"name"`

	Description string `json:"description"`

	Banner string `json:"banner"`

	PreferredLocale string `json:"preferred_locale"`

	// The hash of the guild's icon. Use Session.GuildIcon
	// to retrieve the icon itself.
	Icon string `json:"icon"`

	// The voice region of the guild.
	Region string `json:"region"`

	// The ID of the AFK voice channel.
	AfkChannelID int64 `json:"afk_channel_id,string"`

	// The user ID of the owner of the guild.
	OwnerID int64 `json:"owner_id,string"`

	// The time at which the current user joined the guild.
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	JoinedAt Timestamp `json:"joined_at"`

	// The hash of the guild's splash.
	Splash string `json:"splash"`

	// The timeout, in seconds, before a user is considered AFK in voice.
	AfkTimeout int `json:"afk_timeout"`

	// The number of members in the guild.
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	MemberCount int `json:"member_count"`

	// The verification level required for the guild.
	VerificationLevel VerificationLevel `json:"verification_level"`

	// Whether the guild is considered large. This is
	// determined by a member threshold in the identify packet,
	// and is currently hard-coded at 250 members in the library.
	Large bool `json:"large"`

	// The default message notification setting for the guild.
	// 0 == all messages, 1 == mentions only.
	DefaultMessageNotifications int `json:"default_message_notifications"`

	// A list of roles in the guild.
	Roles []*Role `json:"roles"`

	// A list of the custom emojis present in the guild.
	Emojis []*Emoji `json:"emojis"`

	// A list of the custom stickers present in the guild.
	Stickers []*Sticker `json:"stickers"`

	// A list of the members in the guild.
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	Members []*Member `json:"members"`

	// A list of partial presence objects for members in the guild.
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	Presences []*Presence `json:"presences"`

	// A list of channels in the guild.
	// This field is only present in GUILD_CREATE events
	Channels []*Channel `json:"channels"`

	// All active threads in the guild that current user has permission to view
	// This field is only present in GUILD_CREATE events
	Threads []*Channel `json:"threads"`

	// A list of voice states for the guild.
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	VoiceStates  []*VoiceState `json:"voice_states"`
	MaxPresences int           `json:"max_presences"`
	MaxMembers   int           `json:"max_members"`

	// Whether this guild is currently unavailable (most likely due to outage).
	// This field is only present in GUILD_CREATE events and websocket
	// update events, and thus is only present in state-cached guilds.
	Unavailable bool `json:"unavailable"`

	// The explicit content filter level
	ExplicitContentFilter ExplicitContentFilterLevel `json:"explicit_content_filter"`

	// The list of enabled guild features
	Features []string `json:"features"`

	// Required MFA level for the guild
	MfaLevel MfaLevel `json:"mfa_level"`

	// Whether or not the Server Widget is enabled
	WidgetEnabled bool `json:"widget_enabled"`

	// The Channel ID for the Server Widget
	WidgetChannelID string `json:"widget_channel_id"`

	// The Channel ID to which system messages are sent (eg join and leave messages)
	SystemChannelID string `json:"system_channel_id"`

	ApproximateMemberCount   int    `json:"approximate_member_count"`
	ApproximatePresenceCount int    `json:"approximate_presence_count"`
	VanityURLCode            string `json:"vanity_url_code"`
}

func (g *Guild) GetGuildID() int64 {
	return g.ID
}

func (g *Guild) Role(id int64) *Role {
	for _, v := range g.Roles {
		if v.ID == id {
			return v
		}
	}

	return nil
}

func (g *Guild) Channel(id int64) *Channel {
	for _, v := range g.Channels {
		if v.ID == id {
			return v
		}
	}

	return nil
}

// A UserGuild holds a brief version of a Guild
type UserGuild struct {
	ID          int64  `json:"id,string"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Owner       bool   `json:"owner"`
	Permissions int64  `json:"permissions,string"`
}

// A GuildParams stores all the data needed to update discord guild settings
type GuildParams struct {
	Name                        string             `json:"name,omitempty"`
	Region                      string             `json:"region,omitempty"`
	VerificationLevel           *VerificationLevel `json:"verification_level,omitempty"`
	DefaultMessageNotifications int                `json:"default_message_notifications,omitempty"` // TODO: Separate type?
	AfkChannelID                int64              `json:"afk_channel_id,omitempty,string"`
	AfkTimeout                  int                `json:"afk_timeout,omitempty"`
	Icon                        string             `json:"icon,omitempty"`
	OwnerID                     int64              `json:"owner_id,omitempty,string"`
	Splash                      string             `json:"splash,omitempty"`
}

// A Role stores information about Discord guild member roles.
type Role struct {
	// The ID of the role.
	ID int64 `json:"id,string"`

	// The name of the role.
	Name string `json:"name"`

	// Whether this role is managed by an integration, and
	// thus cannot be manually added to, or taken from, members.
	Managed bool `json:"managed"`

	// Whether this role is mentionable.
	Mentionable bool `json:"mentionable"`

	// Whether this role is hoisted (shows up separately in member list).
	Hoist bool `json:"hoist"`

	// The hex color of this role.
	Color int `json:"color"`

	// The position of this role in the guild's role hierarchy.
	Position int `json:"position"`

	// The permissions of the role on the guild (doesn't include channel overrides).
	// This is a combination of bit masks; the presence of a certain permission can
	// be checked by performing a bitwise AND between this int and the permission.
	Permissions int64 `json:"permissions,string"`

	// The hash of the role icon. Use .IconURL to retrieve the icon's URL.
	Icon string `json:"icon"`
}

// Mention returns a string which mentions the role
func (r *Role) Mention() string {
	return fmt.Sprintf("<@&%d>", r.ID)
}

func (r *Role) IconURL(size string) string {
	if r.Icon == "" {
		return ""
	}

	URL := EndpointRoleIcon(r.ID, r.Icon)

	if size != "" {
		return URL + "?size=" + size
	}

	return URL
}

// Roles are a collection of Role
type Roles []*Role

func (r Roles) Len() int {
	return len(r)
}

func (r Roles) Less(i, j int) bool {
	return r[i].Position > r[j].Position
}

func (r Roles) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// A VoiceState stores the voice states of Guilds
type VoiceState struct {
	UserID     int64  `json:"user_id,string"`
	SessionID  string `json:"session_id"`
	ChannelID  int64  `json:"channel_id,string"`
	GuildID    int64  `json:"guild_id,string"`
	Suppress   bool   `json:"suppress"`
	SelfMute   bool   `json:"self_mute"`
	SelfDeaf   bool   `json:"self_deaf"`
	Mute       bool   `json:"mute"`
	Deaf       bool   `json:"deaf"`
	SelfStream bool   `json:"self_stream"`
	SelfVideo  bool   `json:"self_video"`
}

// A Presence stores the online, offline, or idle and game status of Guild members.
type Presence struct {
	User   *User  `json:"user"`
	Status Status `json:"status"`

	Activities Activities `json:"activities"`
}

// implement gojay.UnmarshalerJSONObject
func (p *Presence) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	var err error
	switch key {
	case "user":
		p.User = &User{}
		err = dec.Object(p.User)
	case "status":
		err = dec.String((*string)(&p.Status))
	case "activities":
		err = dec.DecodeArray(&p.Activities)
	default:
	}

	if err != nil {
		return errors.Wrap(err, key)
	}

	return nil
}

func (p *Presence) NKeys() int {
	return 0
}

type Activities []*Activity

func (a *Activities) UnmarshalJSONArray(dec *gojay.Decoder) error {
	instance := Activity{}
	err := dec.Object(&instance)
	if err != nil {
		return err
	}
	*a = append(*a, &instance)
	return nil
}

// ActivityType is the type of presence (see ActivityType* consts) in the Activity struct
type ActivityType int

// Valid ActivityType values
const (
	ActivityTypePlaying ActivityType = iota
	ActivityTypeStreaming
	ActivityTypeListening
	ActivityTypeWatching
	ActivityTypeCustom
	ActivityTypeCompeting
)

// An Activity struct holds data about a user's activity.
type Activity struct {
	Name       string       `json:"name"`
	Type       ActivityType `json:"type"`
	URL        string       `json:"url,omitempty"`
	Details    string       `json:"details,omitempty"`
	State      string       `json:"state,omitempty"`
	TimeStamps TimeStamps   `json:"timestamps,omitempty"`
	Assets     Assets       `json:"assets,omitempty"`
	Instance   int8         `json:"instance,omitempty"`
}

// implement gojay.UnmarshalerJSONObject
func (a *Activity) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "name":
		return dec.String(&a.Name)
	case "type":
		return dec.Int((*int)(&a.Type))
	case "url":
		return dec.String(&a.URL)
	case "details":
		return dec.String(&a.Details)
	case "state":
		return dec.String(&a.State)
	case "timestamps":
		return dec.Object(&a.TimeStamps)
	case "assets":
	case "instance":
		return dec.Int8(&a.Instance)
	}

	return nil
}

func (a *Activity) NKeys() int {
	return 0
}

// A TimeStamps struct contains start and end times used in the rich presence "playing .." Game
type TimeStamps struct {
	EndTimestamp   int64 `json:"end,omitempty"`
	StartTimestamp int64 `json:"start,omitempty"`
}

func (t *TimeStamps) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "start":
		return dec.Int64(&t.StartTimestamp)
	case "end":
		return dec.Int64(&t.EndTimestamp)
	}

	return nil
}

// UnmarshalJSON unmarshals JSON into TimeStamps struct
func (t *TimeStamps) UnmarshalJSON(b []byte) error {
	temp := struct {
		End   json.Number `json:"end,omitempty"`
		Start json.Number `json:"start,omitempty"`
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	var endParsed float64
	if temp.End != "" {
		endParsed, err = temp.End.Float64()
		if err != nil {
			return err
		}
	}

	var startParsed float64
	if temp.Start != "" {
		startParsed, err = temp.Start.Float64()
		if err != nil {
			return err
		}
	}

	t.EndTimestamp = int64(endParsed)
	t.StartTimestamp = int64(startParsed)
	return nil
}

func (t *TimeStamps) NKeys() int {
	return 0
}

// An Assets struct contains assets and labels used in the rich presence "playing .." Game
type Assets struct {
	LargeImageID string `json:"large_image,omitempty"`
	SmallImageID string `json:"small_image,omitempty"`
	LargeText    string `json:"large_text,omitempty"`
	SmallText    string `json:"small_text,omitempty"`
}

// A MessageActivity represents the activity sent with a message, such as a game invite.
type MessageActivity struct {
	Type    int    `json:"type"`
	PartyID string `json:"party_id"`
}

// MemberFlags represent flags of a guild member.
// https://discord.com/developers/docs/resources/guild#guild-member-object-guild-member-flags
type MemberFlags int

// Block containing known MemberFlags values.
const (
	// MemberFlagDidRejoin indicates whether the Member has left and rejoined the guild.
	MemberFlagDidRejoin MemberFlags = 1 << 0
	// MemberFlagCompletedOnboarding indicates whether the Member has completed onboarding.
	MemberFlagCompletedOnboarding MemberFlags = 1 << 1
	// MemberFlagBypassesVerification indicates whether the Member is exempt from guild verification requirements.
	MemberFlagBypassesVerification MemberFlags = 1 << 2
	// MemberFlagStartedOnboarding indicates whether the Member has started onboarding.
	MemberFlagStartedOnboarding MemberFlags = 1 << 3
)

// A Member stores user information for Guild members. A guild
// member represents a certain user's presence in a guild.
type Member struct {
	// The guild ID on which the member exists.
	GuildID int64 `json:"guild_id,string"`

	// The time at which the member joined the guild, in ISO8601.
	JoinedAt Timestamp `json:"joined_at"`

	// The nickname of the member, if they have one.
	Nick string `json:"nick"`

	// The guild avatar hash of the member, if they have one.
	Avatar string `json:"avatar"`

	// The guild banner hash of the member, if they have one.
	Banner string `json:"banner"`

	// Whether the member is deafened at a guild level.
	Deaf bool `json:"deaf"`

	// Whether the member is muted at a guild level.
	Mute bool `json:"mute"`

	// The underlying user on which the member is based.
	User *User `json:"user"`

	// A list of IDs of the roles which are possessed by the member.
	Roles IDSlice `json:"roles,string"`

	// When the user used their Nitro boost on the server
	PremiumSince *time.Time `json:"premium_since"`

	// The flags of this member. This is a combination of bit masks; the presence of a certain
	// flag can be checked by performing a bitwise AND between this int and the flag.
	Flags MemberFlags `json:"flags"`

	// Whether the user has not yet passed the guild's Membership Screening requirements
	Pending bool `json:"pending"`

	// The time at which the member's timeout will expire.
	// Time in the past or nil if the user is not timed out.
	CommunicationDisabledUntil *time.Time `json:"communication_disabled_until"`
}

func (m *Member) GetGuildID() int64 {
	return m.GuildID
}

func (m *Member) AvatarURL(size string) string {
	var URL string

	if m == nil {
		return "Member not found"
	}

	u := m.User

	if m.Avatar == "" {
		return u.AvatarURL(size)
	} else if strings.HasPrefix(m.Avatar, "a_") {
		URL = EndpointGuildMemberAvatarAnimated(m.GuildID, u.ID, m.Avatar)
	} else {
		URL = EndpointGuildMemberAvatar(m.GuildID, u.ID, m.Avatar)
	}

	if size != "" {
		return URL + "?size=" + size
	}
	return URL
}

func (m *Member) BannerURL(size string) string {
	var URL string
	if m.Banner == "" {
		return ""
	} else if strings.HasPrefix(m.Banner, "a_") {
		URL = EndpointUserBannerAnimated(m.GuildID, m.Banner)
	} else {
		URL = EndpointUserBanner(m.GuildID, m.Banner)
	}

	if size != "" {
		return URL + "?size=" + size
	}
	return URL
}

// A Settings stores data for a specific users Discord client settings.
type Settings struct {
	RenderEmbeds           bool               `json:"render_embeds"`
	InlineEmbedMedia       bool               `json:"inline_embed_media"`
	InlineAttachmentMedia  bool               `json:"inline_attachment_media"`
	EnableTtsCommand       bool               `json:"enable_tts_command"`
	MessageDisplayCompact  bool               `json:"message_display_compact"`
	ShowCurrentGame        bool               `json:"show_current_game"`
	ConvertEmoticons       bool               `json:"convert_emoticons"`
	Locale                 string             `json:"locale"`
	Theme                  string             `json:"theme"`
	GuildPositions         IDSlice            `json:"guild_positions,string"`
	RestrictedGuilds       IDSlice            `json:"restricted_guilds,string"`
	FriendSourceFlags      *FriendSourceFlags `json:"friend_source_flags"`
	Status                 Status             `json:"status"`
	DetectPlatformAccounts bool               `json:"detect_platform_accounts"`
	DeveloperMode          bool               `json:"developer_mode"`
}

// Status type definition
type Status string

// Constants for Status with the different current available status
const (
	StatusOnline       Status = "online"
	StatusIdle         Status = "idle"
	StatusDoNotDisturb Status = "dnd"
	StatusInvisible    Status = "invisible"
	StatusOffline      Status = "offline"
)

// FriendSourceFlags stores ... TODO :)
type FriendSourceFlags struct {
	All           bool `json:"all"`
	MutualGuilds  bool `json:"mutual_guilds"`
	MutualFriends bool `json:"mutual_friends"`
}

// A Relationship between the logged in user and Relationship.User
type Relationship struct {
	User *User  `json:"user"`
	Type int    `json:"type"` // 1 = friend, 2 = blocked, 3 = incoming friend req, 4 = sent friend req
	ID   string `json:"id"`
}

// A TooManyRequests struct holds information received from Discord
// when receiving a HTTP 429 response.
type TooManyRequests struct {
	Bucket     string  `json:"bucket"`
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
}

func (t *TooManyRequests) RetryAfterDur() time.Duration {
	return time.Duration(t.RetryAfter*1000) * time.Millisecond
}

// A ReadState stores data on the read state of channels.
type ReadState struct {
	MentionCount  int   `json:"mention_count"`
	LastMessageID int64 `json:"last_message_id,string"`
	ID            int64 `json:"id,string"`
}

// An Ack is used to ack messages
type Ack struct {
	Token string `json:"token"`
}

// A GuildRole stores data for guild roles.
type GuildRole struct {
	Role    *Role `json:"role"`
	GuildID int64 `json:"guild_id,string"`
}

func (e *GuildRole) GetGuildID() int64 {
	return e.GuildID
}

// A GuildBan stores data for a guild ban.
type GuildBan struct {
	Reason string `json:"reason"`
	User   *User  `json:"user"`
}

// A GuildEmbed stores data for a guild embed.
type GuildEmbed struct {
	Enabled   bool  `json:"enabled"`
	ChannelID int64 `json:"channel_id,string"`
}

// A GuildAuditLog stores data for a guild audit log.
type GuildAuditLog struct {
	Webhooks []struct {
		ChannelID int64  `json:"channel_id,string"`
		GuildID   int64  `json:"guild_id,string"`
		ID        string `json:"id"`
		Avatar    string `json:"avatar"`
		Name      string `json:"name"`
	} `json:"webhooks,omitempty"`
	Users           []*User          `json:"users,omitempty"`
	AuditLogEntries []*AuditLogEntry `json:"audit_log_entries"`
}

// AuditLogAction is the Action of the AuditLog (see AuditLogAction* consts)
// https://discord.com/developers/docs/resources/audit-log#audit-log-entry-object-audit-log-events
type AuditLogAction int

// Block contains Discord Audit Log Action Types
const (
	AuditLogActionGuildUpdate AuditLogAction = 1

	AuditLogActionChannelCreate          AuditLogAction = 10
	AuditLogActionChannelUpdate          AuditLogAction = 11
	AuditLogActionChannelDelete          AuditLogAction = 12
	AuditLogActionChannelOverwriteCreate AuditLogAction = 13
	AuditLogActionChannelOverwriteUpdate AuditLogAction = 14
	AuditLogActionChannelOverwriteDelete AuditLogAction = 15

	AuditLogActionMemberKick       AuditLogAction = 20
	AuditLogActionMemberPrune      AuditLogAction = 21
	AuditLogActionMemberBanAdd     AuditLogAction = 22
	AuditLogActionMemberBanRemove  AuditLogAction = 23
	AuditLogActionMemberUpdate     AuditLogAction = 24
	AuditLogActionMemberRoleUpdate AuditLogAction = 25
	AuditLogActionMemberMove       AuditLogAction = 26
	AuditLogActionMemberDisconnect AuditLogAction = 27
	AuditLogActionBotAdd           AuditLogAction = 28

	AuditLogActionRoleCreate AuditLogAction = 30
	AuditLogActionRoleUpdate AuditLogAction = 31
	AuditLogActionRoleDelete AuditLogAction = 32

	AuditLogActionInviteCreate AuditLogAction = 40
	AuditLogActionInviteUpdate AuditLogAction = 41
	AuditLogActionInviteDelete AuditLogAction = 42

	AuditLogActionWebhookCreate AuditLogAction = 50
	AuditLogActionWebhookUpdate AuditLogAction = 51
	AuditLogActionWebhookDelete AuditLogAction = 52

	AuditLogActionEmojiCreate AuditLogAction = 60
	AuditLogActionEmojiUpdate AuditLogAction = 61
	AuditLogActionEmojiDelete AuditLogAction = 62

	AuditLogActionMessageDelete     AuditLogAction = 72
	AuditLogActionMessageBulkDelete AuditLogAction = 73
	AuditLogActionMessagePin        AuditLogAction = 74
	AuditLogActionMessageUnpin      AuditLogAction = 75

	AuditLogActionIntegrationCreate   AuditLogAction = 80
	AuditLogActionIntegrationUpdate   AuditLogAction = 81
	AuditLogActionIntegrationDelete   AuditLogAction = 82
	AuditLogActionStageInstanceCreate AuditLogAction = 83
	AuditLogActionStageInstanceUpdate AuditLogAction = 84
	AuditLogActionStageInstanceDelete AuditLogAction = 85

	AuditLogActionStickerCreate AuditLogAction = 90
	AuditLogActionStickerUpdate AuditLogAction = 91
	AuditLogActionStickerDelete AuditLogAction = 92

	AuditLogGuildScheduledEventCreate AuditLogAction = 100
	AuditLogGuildScheduledEventUpdare AuditLogAction = 101
	AuditLogGuildScheduledEventDelete AuditLogAction = 102

	AuditLogActionThreadCreate AuditLogAction = 110
	AuditLogActionThreadUpdate AuditLogAction = 111
	AuditLogActionThreadDelete AuditLogAction = 112

	AuditLogActionApplicationCommandPermissionUpdate      AuditLogAction = 121
	AuditLogActionAutoModerationRuleCreate                AuditLogAction = 140
	AuditLogActionAutoModerationRuleUpdate                AuditLogAction = 141
	AuditLogActionAutoModerationRuleDelete                AuditLogAction = 142
	AuditLogActionAutoModerationBlockMessage              AuditLogAction = 143
	AuditLogActionAutoModerationFlagToChannel             AuditLogAction = 144
	AuditLogActionAutoModerationUserCommunicationDisabled AuditLogAction = 145
)

// AuditLogChange for an AuditLogEntry
type AuditLogChange struct {
	NewValue interface{}        `json:"new_value"`
	OldValue interface{}        `json:"old_value"`
	Key      *AuditLogChangeKey `json:"key"`
}

// AuditLogChangeKey value for AuditLogChange
// https://discord.com/developers/docs/resources/audit-log#audit-log-change-object-audit-log-change-key
type AuditLogChangeKey string

// Block of valid AuditLogChangeKey
const (
	// AuditLogChangeKeyAfkChannelID is sent when afk channel changed (snowflake) - guild
	AuditLogChangeKeyAfkChannelID AuditLogChangeKey = "afk_channel_id"
	// AuditLogChangeKeyAfkTimeout is sent when afk timeout duration changed (int) - guild
	AuditLogChangeKeyAfkTimeout AuditLogChangeKey = "afk_timeout"
	// AuditLogChangeKeyAllow is sent when a permission on a text or voice channel was allowed for a role (string) - role
	AuditLogChangeKeyAllow AuditLogChangeKey = "allow"
	// AudirChangeKeyApplicationID is sent when application id of the added or removed webhook or bot (snowflake) - channel
	AuditLogChangeKeyApplicationID AuditLogChangeKey = "application_id"
	// AuditLogChangeKeyArchived is sent when thread was archived/unarchived (bool) - thread
	AuditLogChangeKeyArchived AuditLogChangeKey = "archived"
	// AuditLogChangeKeyAsset is sent when asset is changed (string) - sticker
	AuditLogChangeKeyAsset AuditLogChangeKey = "asset"
	// AuditLogChangeKeyAutoArchiveDuration is sent when auto archive duration changed (int) - thread
	AuditLogChangeKeyAutoArchiveDuration AuditLogChangeKey = "auto_archive_duration"
	// AuditLogChangeKeyAvailable is sent when availability of sticker changed (bool) - sticker
	AuditLogChangeKeyAvailable AuditLogChangeKey = "available"
	// AuditLogChangeKeyAvatarHash is sent when user avatar changed (string) - user
	AuditLogChangeKeyAvatarHash AuditLogChangeKey = "avatar_hash"
	// AuditLogChangeKeyBannerHash is sent when guild banner changed (string) - guild
	AuditLogChangeKeyBannerHash AuditLogChangeKey = "banner_hash"
	// AuditLogChangeKeyBitrate is sent when voice channel bitrate changed (int) - channel
	AuditLogChangeKeyBitrate AuditLogChangeKey = "bitrate"
	// AuditLogChangeKeyChannelID is sent when channel for invite code or guild scheduled event changed (snowflake) - invite or guild scheduled event
	AuditLogChangeKeyChannelID AuditLogChangeKey = "channel_id"
	// AuditLogChangeKeyCode is sent when invite code changed (string) - invite
	AuditLogChangeKeyCode AuditLogChangeKey = "code"
	// AuditLogChangeKeyColor is sent when role color changed (int) - role
	AuditLogChangeKeyColor AuditLogChangeKey = "color"
	// AuditLogChangeKeyCommunicationDisabledUntil is sent when member timeout state changed (ISO8601 timestamp) - member
	AuditLogChangeKeyCommunicationDisabledUntil AuditLogChangeKey = "communication_disabled_until"
	// AuditLogChangeKeyDeaf is sent when user server deafened/undeafened (bool) - member
	AuditLogChangeKeyDeaf AuditLogChangeKey = "deaf"
	// AuditLogChangeKeyDefaultAutoArchiveDuration is sent when default auto archive duration for newly created threads changed (int) - channel
	AuditLogChangeKeyDefaultAutoArchiveDuration AuditLogChangeKey = "default_auto_archive_duration"
	// AuditLogChangeKeyDefaultMessageNotification is sent when default message notification level changed (int) - guild
	AuditLogChangeKeyDefaultMessageNotification AuditLogChangeKey = "default_message_notifications"
	// AuditLogChangeKeyDeny is sent when a permission on a text or voice channel was denied for a role (string) - role
	AuditLogChangeKeyDeny AuditLogChangeKey = "deny"
	// AuditLogChangeKeyDescription is sent when description changed (string) - guild, sticker, or guild scheduled event
	AuditLogChangeKeyDescription AuditLogChangeKey = "description"
	// AuditLogChangeKeyDiscoverySplashHash is sent when discovery splash changed (string) - guild
	AuditLogChangeKeyDiscoverySplashHash AuditLogChangeKey = "discovery_splash_hash"
	// AuditLogChangeKeyEnableEmoticons is sent when integration emoticons enabled/disabled (bool) - integration
	AuditLogChangeKeyEnableEmoticons AuditLogChangeKey = "enable_emoticons"
	// AuditLogChangeKeyEntityType is sent when entity type of guild scheduled event was changed (int) - guild scheduled event
	AuditLogChangeKeyEntityType AuditLogChangeKey = "entity_type"
	// AuditLogChangeKeyExpireBehavior is sent when integration expiring subscriber behavior changed (int) - integration
	AuditLogChangeKeyExpireBehavior AuditLogChangeKey = "expire_behavior"
	// AuditLogChangeKeyExpireGracePeriod is sent when integration expire grace period changed (int) - integration
	AuditLogChangeKeyExpireGracePeriod AuditLogChangeKey = "expire_grace_period"
	// AuditLogChangeKeyExplicitContentFilter is sent when change in whose messages are scanned and deleted for explicit content in the server is made (int) - guild
	AuditLogChangeKeyExplicitContentFilter AuditLogChangeKey = "explicit_content_filter"
	// AuditLogChangeKeyFormatType is sent when format type of sticker changed (int - sticker format type) - sticker
	AuditLogChangeKeyFormatType AuditLogChangeKey = "format_type"
	// AuditLogChangeKeyGuildID is sent when guild sticker is in changed (snowflake) - sticker
	AuditLogChangeKeyGuildID AuditLogChangeKey = "guild_id"
	// AuditLogChangeKeyHoist is sent when role is now displayed/no longer displayed separate from online users (bool) - role
	AuditLogChangeKeyHoist AuditLogChangeKey = "hoist"
	// AuditLogChangeKeyIconHash is sent when icon changed (string) - guild or role
	AuditLogChangeKeyIconHash AuditLogChangeKey = "icon_hash"
	// AuditLogChangeKeyID is sent when the id of the changed entity - sometimes used in conjunction with other keys (snowflake) - any
	AuditLogChangeKeyID AuditLogChangeKey = "id"
	// AuditLogChangeKeyInvitable is sent when private thread is now invitable/uninvitable (bool) - thread
	AuditLogChangeKeyInvitable AuditLogChangeKey = "invitable"
	// AuditLogChangeKeyInviterID is sent when person who created invite code changed (snowflake) - invite
	AuditLogChangeKeyInviterID AuditLogChangeKey = "inviter_id"
	// AuditLogChangeKeyLocation is sent when channel id for guild scheduled event changed (string) - guild scheduled event
	AuditLogChangeKeyLocation AuditLogChangeKey = "location"
	// AuditLogChangeKeyLocked is sent when thread was locked/unlocked (bool) - thread
	AuditLogChangeKeyLocked AuditLogChangeKey = "locked"
	// AuditLogChangeKeyMaxAge is sent when invite code expiration time changed (int) - invite
	AuditLogChangeKeyMaxAge AuditLogChangeKey = "max_age"
	// AuditLogChangeKeyMaxUses is sent when max number of times invite code can be used changed (int) - invite
	AuditLogChangeKeyMaxUses AuditLogChangeKey = "max_uses"
	// AuditLogChangeKeyMentionable is sent when role is now mentionable/unmentionable (bool) - role
	AuditLogChangeKeyMentionable AuditLogChangeKey = "mentionable"
	// AuditLogChangeKeyMfaLevel is sent when two-factor auth requirement changed (int - mfa level) - guild
	AuditLogChangeKeyMfaLevel AuditLogChangeKey = "mfa_level"
	// AuditLogChangeKeyMute is sent when user server muted/unmuted (bool) - member
	AuditLogChangeKeyMute AuditLogChangeKey = "mute"
	// AuditLogChangeKeyName is sent when name changed (string) - any
	AuditLogChangeKeyName AuditLogChangeKey = "name"
	// AuditLogChangeKeyNick is sent when user nickname changed (string) - member
	AuditLogChangeKeyNick AuditLogChangeKey = "nick"
	// AuditLogChangeKeyNSFW is sent when channel nsfw restriction changed (bool) - channel
	AuditLogChangeKeyNSFW AuditLogChangeKey = "nsfw"
	// AuditLogChangeKeyOwnerID is sent when owner changed (snowflake) - guild
	AuditLogChangeKeyOwnerID AuditLogChangeKey = "owner_id"
	// AuditLogChangeKeyPermissionOverwrite is sent when permissions on a channel changed (array of channel overwrite objects) - channel
	AuditLogChangeKeyPermissionOverwrite AuditLogChangeKey = "permission_overwrites"
	// AuditLogChangeKeyPermissions is sent when permissions for a role changed (string) - role
	AuditLogChangeKeyPermissions AuditLogChangeKey = "permissions"
	// AuditLogChangeKeyPosition is sent when text or voice channel position changed (int) - channel
	AuditLogChangeKeyPosition AuditLogChangeKey = "position"
	// AuditLogChangeKeyPreferredLocale is sent when preferred locale changed (string) - guild
	AuditLogChangeKeyPreferredLocale AuditLogChangeKey = "preferred_locale"
	// AuditLogChangeKeyPrivacylevel is sent when privacy level of the stage instance changed (integer - privacy level) - stage instance or guild scheduled event
	AuditLogChangeKeyPrivacylevel AuditLogChangeKey = "privacy_level"
	// AuditLogChangeKeyPruneDeleteDays is sent when number of days after which inactive and role-unassigned members are kicked changed (int) - guild
	AuditLogChangeKeyPruneDeleteDays AuditLogChangeKey = "prune_delete_days"
	// AuditLogChangeKeyPulibUpdatesChannelID is sent when id of the public updates channel changed (snowflake) - guild
	AuditLogChangeKeyPulibUpdatesChannelID AuditLogChangeKey = "public_updates_channel_id"
	// AuditLogChangeKeyRateLimitPerUser is sent when amount of seconds a user has to wait before sending another message changed (int) - channel
	AuditLogChangeKeyRateLimitPerUser AuditLogChangeKey = "rate_limit_per_user"
	// AuditLogChangeKeyRegion is sent when region changed (string) - guild
	AuditLogChangeKeyRegion AuditLogChangeKey = "region"
	// AuditLogChangeKeyRulesChannelID is sent when id of the rules channel changed (snowflake) - guild
	AuditLogChangeKeyRulesChannelID AuditLogChangeKey = "rules_channel_id"
	// AuditLogChangeKeySplashHash is sent when invite splash page artwork changed (string) - guild
	AuditLogChangeKeySplashHash AuditLogChangeKey = "splash_hash"
	// AuditLogChangeKeyStatus is sent when status of guild scheduled event was changed (int - guild scheduled event status) - guild scheduled event
	AuditLogChangeKeyStatus AuditLogChangeKey = "status"
	// AuditLogChangeKeySystemChannelID is sent when id of the system channel changed (snowflake) - guild
	AuditLogChangeKeySystemChannelID AuditLogChangeKey = "system_channel_id"
	// AuditLogChangeKeyTags is sent when related emoji of sticker changed (string) - sticker
	AuditLogChangeKeyTags AuditLogChangeKey = "tags"
	// AuditLogChangeKeyTemporary is sent when invite code is now temporary or never expires (bool) - invite
	AuditLogChangeKeyTemporary AuditLogChangeKey = "temporary"
	// TODO: remove when compatibility is not required
	AuditLogChangeKeyTempoary = AuditLogChangeKeyTemporary
	// AuditLogChangeKeyTopic is sent when text channel topic or stage instance topic changed (string) - channel or stage instance
	AuditLogChangeKeyTopic AuditLogChangeKey = "topic"
	// AuditLogChangeKeyType is sent when type of entity created (int or string) - any
	AuditLogChangeKeyType AuditLogChangeKey = "type"
	// AuditLogChangeKeyUnicodeEmoji is sent when role unicode emoji changed (string) - role
	AuditLogChangeKeyUnicodeEmoji AuditLogChangeKey = "unicode_emoji"
	// AuditLogChangeKeyUserLimit is sent when new user limit in a voice channel set (int) - voice channel
	AuditLogChangeKeyUserLimit AuditLogChangeKey = "user_limit"
	// AuditLogChangeKeyUses is sent when number of times invite code used changed (int) - invite
	AuditLogChangeKeyUses AuditLogChangeKey = "uses"
	// AuditLogChangeKeyVanityURLCode is sent when guild invite vanity url changed (string) - guild
	AuditLogChangeKeyVanityURLCode AuditLogChangeKey = "vanity_url_code"
	// AuditLogChangeKeyVerificationLevel is sent when required verification level changed (int - verification level) - guild
	AuditLogChangeKeyVerificationLevel AuditLogChangeKey = "verification_level"
	// AuditLogChangeKeyWidgetChannelID is sent when channel id of the server widget changed (snowflake) - guild
	AuditLogChangeKeyWidgetChannelID AuditLogChangeKey = "widget_channel_id"
	// AuditLogChangeKeyWidgetEnabled is sent when server widget enabled/disabled (bool) - guild
	AuditLogChangeKeyWidgetEnabled AuditLogChangeKey = "widget_enabled"
	// AuditLogChangeKeyRoleAdd is sent when new role added (array of partial role objects) - guild
	AuditLogChangeKeyRoleAdd AuditLogChangeKey = "$add"
	// AuditLogChangeKeyRoleRemove is sent when role removed (array of partial role objects) - guild
	AuditLogChangeKeyRoleRemove AuditLogChangeKey = "$remove"
)

// AuditLogOptions optional data for the AuditLog
// https://discord.com/developers/docs/resources/audit-log#audit-log-entry-object-optional-audit-entry-info
type AuditLogOptions struct {
	DeleteMemberDays string               `json:"delete_member_days"`
	MembersRemoved   string               `json:"members_removed"`
	ChannelID        string               `json:"channel_id"`
	MessageID        string               `json:"message_id"`
	Count            string               `json:"count"`
	ID               string               `json:"id"`
	Type             *AuditLogOptionsType `json:"type"`
	RoleName         string               `json:"role_name"`
}

// AuditLogOptionsType of the AuditLogOption
// https://discord.com/developers/docs/resources/audit-log#audit-log-entry-object-optional-audit-entry-info
type AuditLogOptionsType string

// Valid Types for AuditLogOptionsType
const (
	AuditLogOptionsTypeMember AuditLogOptionsType = "member"
	AuditLogOptionsTypeRole   AuditLogOptionsType = "role"
)

// AuditLogEntry for a GuildAuditLog
// https://discord.com/developers/docs/resources/audit-log#audit-log-entry-object-audit-log-entry-structure
type AuditLogEntry struct {
	TargetID   int64             `json:"target_id,string"`
	Changes    []*AuditLogChange `json:"changes"`
	UserID     int64             `json:"user_id,string"`
	ID         int64             `json:"id,string"`
	ActionType *AuditLogAction   `json:"action_type"`
	Options    *AuditLogOptions  `json:"options"`
	Reason     string            `json:"reason"`
	GuildID    int64             `json:"guild_id,string,omitempty"`
}

// A UserGuildSettingsChannelOverride stores data for a channel override for a users guild settings.
type UserGuildSettingsChannelOverride struct {
	Muted                bool  `json:"muted"`
	MessageNotifications int   `json:"message_notifications"`
	ChannelID            int64 `json:"channel_id,string"`
}

// A UserGuildSettings stores data for a users guild settings.
type UserGuildSettings struct {
	SupressEveryone      bool                                `json:"suppress_everyone"`
	Muted                bool                                `json:"muted"`
	MobilePush           bool                                `json:"mobile_push"`
	MessageNotifications int                                 `json:"message_notifications"`
	GuildID              int64                               `json:"guild_id,string"`
	ChannelOverrides     []*UserGuildSettingsChannelOverride `json:"channel_overrides"`
}

// A UserGuildSettingsEdit stores data for editing UserGuildSettings
type UserGuildSettingsEdit struct {
	SupressEveryone      bool                                         `json:"suppress_everyone"`
	Muted                bool                                         `json:"muted"`
	MobilePush           bool                                         `json:"mobile_push"`
	MessageNotifications int                                          `json:"message_notifications"`
	ChannelOverrides     map[string]*UserGuildSettingsChannelOverride `json:"channel_overrides"`
}

// An APIErrorMessage is an api error message returned from discord
type APIErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Webhook stores the data for a webhook.
type Webhook struct {
	ID        int64  `json:"id,string"`
	GuildID   int64  `json:"guild_id,string"`
	ChannelID int64  `json:"channel_id,string"`
	User      *User  `json:"user"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	Token     string `json:"token"`
}

// WebhookParams is a struct for webhook params, used in the WebhookExecute command.
type WebhookParams struct {
	Content         string              `json:"content,omitempty"`
	Username        string              `json:"username,omitempty"`
	AvatarURL       string              `json:"avatar_url,omitempty"`
	TTS             bool                `json:"tts,omitempty"`
	File            *File               `json:"-,omitempty"`
	Components      []TopLevelComponent `json:"components"`
	Embeds          []*MessageEmbed     `json:"embeds,omitempty"`
	Flags           int64               `json:"flags,omitempty"`
	AllowedMentions *AllowedMentions    `json:"allowed_mentions,omitempty"`
}

// MessageReaction stores the data for a message reaction.
type MessageReaction struct {
	UserID    int64 `json:"user_id,string"`
	MessageID int64 `json:"message_id,string"`
	Emoji     Emoji `json:"emoji"`
	ChannelID int64 `json:"channel_id,string"`
	GuildID   int64 `json:"guild_id,string,omitempty"`
}

func (mr *MessageReaction) GetGuildID() int64 {
	return mr.GuildID
}

func (mr *MessageReaction) GetChannelID() int64 {
	return mr.ChannelID
}

// GatewayBotResponse stores the data for the gateway/bot response
type GatewayBotResponse struct {
	URL               string            `json:"url"`
	Shards            int               `json:"shards"`
	SessionStartLimit SessionStartLimit `json:"session_start_limit"`
}

type SessionStartLimit struct {
	Total          int   `json:"total"`
	Remaining      int   `json:"remaining"`
	ResetAfter     int64 `json:"reset_after"`
	MaxConcurrency int   `json:"max_concurrency"`
}

// Block contains Discord JSON Error Response codes
const (
	ErrCodeUnknownAccount     = 10001
	ErrCodeUnknownApplication = 10002
	ErrCodeUnknownChannel     = 10003
	ErrCodeUnknownGuild       = 10004
	ErrCodeUnknownIntegration = 10005
	ErrCodeUnknownInvite      = 10006
	ErrCodeUnknownMember      = 10007
	ErrCodeUnknownMessage     = 10008
	ErrCodeUnknownOverwrite   = 10009
	ErrCodeUnknownProvider    = 10010
	ErrCodeUnknownRole        = 10011
	ErrCodeUnknownToken       = 10012
	ErrCodeUnknownUser        = 10013
	ErrCodeUnknownEmoji       = 10014
	ErrCodeUnknownWebhook     = 10015

	ErrCodeBotsCannotUseEndpoint  = 20001
	ErrCodeOnlyBotsCanUseEndpoint = 20002

	ErrCodeMaximumGuildsReached     = 30001
	ErrCodeMaximumFriendsReached    = 30002
	ErrCodeMaximumPinsReached       = 30003
	ErrCodeMaximumGuildRolesReached = 30005
	ErrCodeTooManyReactions         = 30010

	ErrCodeUnauthorized              = 40001
	ErrCodeMessageAlreadyCrossposted = 40033

	ErrCodeMissingAccess                             = 50001
	ErrCodeInvalidAccountType                        = 50002
	ErrCodeCannotExecuteActionOnDMChannel            = 50003
	ErrCodeEmbedCisabled                             = 50004
	ErrCodeCannotEditFromAnotherUser                 = 50005
	ErrCodeCannotSendEmptyMessage                    = 50006
	ErrCodeCannotSendMessagesToThisUser              = 50007
	ErrCodeCannotSendMessagesInVoiceChannel          = 50008
	ErrCodeChannelVerificationLevelTooHigh           = 50009
	ErrCodeOAuth2ApplicationDoesNotHaveBot           = 50010
	ErrCodeOAuth2ApplicationLimitReached             = 50011
	ErrCodeInvalidOAuthState                         = 50012
	ErrCodeMissingPermissions                        = 50013
	ErrCodeInvalidAuthenticationToken                = 50014
	ErrCodeNoteTooLong                               = 50015
	ErrCodeTooFewOrTooManyMessagesToDelete           = 50016
	ErrCodeCanOnlyPinMessageToOriginatingChannel     = 50019
	ErrCodeCannotExecuteActionOnSystemMessage        = 50021
	ErrCodeMessageProvidedTooOldForBulkDelete        = 50034
	ErrCodeInvalidFormBody                           = 50035
	ErrCodeInviteAcceptedToGuildApplicationsBotNotIn = 50036

	ErrCodeReactionBlocked = 90001
)

// InviteUser is a partial user obejct from the invite event(s)
type InviteUser struct {
	ID            int64  `json:"id,string"`
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
	Username      string `json:"username"`
}

type CreateApplicationCommandRequest struct {
	Name              string                      `json:"name"`                         // 1-32 character name matching ^[\w-]{1,32}$
	Description       string                      `json:"description"`                  // 1-100 character description
	Options           []*ApplicationCommandOption `json:"options"`                      // the parameters for the command
	DefaultPermission *bool                       `json:"default_permission,omitempty"` // (default true)	whether the command is enabled by default when the app is added to a guild
	NSFW              bool                        `json:"nsfw,omitempty"`               // marks a command as age-restricted
}

func (a *ApplicationCommandInteractionDataResolved) UnmarshalJSON(b []byte) error {
	var temp *applicationCommandInteractionDataResolvedTemp
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	*a = ApplicationCommandInteractionDataResolved{
		Users:    make(map[int64]*User),
		Members:  make(map[int64]*Member),
		Roles:    make(map[int64]*Role),
		Channels: make(map[int64]*Channel),
	}

	for k, v := range temp.Channels {
		parsed, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return err
		}
		a.Channels[parsed] = v
	}

	for k, v := range temp.Roles {
		parsed, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return err
		}
		a.Roles[parsed] = v
	}

	for k, v := range temp.Members {
		parsed, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return err
		}
		a.Members[parsed] = v
	}

	for k, v := range temp.Users {
		parsed, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return err
		}
		a.Users[parsed] = v
	}

	return nil
}

type applicationCommandInteractionDataResolvedTemp struct {
	Users    map[string]*User    `json:"users"`
	Members  map[string]*Member  `json:"members"`
	Roles    map[string]*Role    `json:"roles"`
	Channels map[string]*Channel `json:"channels"`
}

type applicationCommandInteractionDataOptionTemporary struct {
	Name    string                                     `json:"name"`    // the name of the parameter
	Type    ApplicationCommandOptionType               `json:"type"`    // value of ApplicationCommandOptionType
	Value   json.RawMessage                            `json:"value"`   // the value of the pair
	Options []*ApplicationCommandInteractionDataOption `json:"options"` // present if this option is a group or subcommand
	Focused bool                                       `json:"focused"` // present if this option is currently focused (in autocomplete)
}

func (a *ApplicationCommandInteractionDataOption) UnmarshalJSON(b []byte) error {
	var temp *applicationCommandInteractionDataOptionTemporary
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	*a = ApplicationCommandInteractionDataOption{
		Name:    temp.Name,
		Type:    temp.Type,
		Options: temp.Options,
		Focused: temp.Focused,
	}

	switch temp.Type {
	case ApplicationCommandOptionString:
		v := ""
		err = json.Unmarshal(temp.Value, &v)
		a.Value = v
	case ApplicationCommandOptionInteger:
		v := int64(0)
		err = json.Unmarshal(temp.Value, &v)
		a.Value = v
	case ApplicationCommandOptionBoolean:
		v := false
		err = json.Unmarshal(temp.Value, &v)
		a.Value = v
	case ApplicationCommandOptionUser, ApplicationCommandOptionChannel, ApplicationCommandOptionRole:
		// parse the snowflake
		v := ""
		err = json.Unmarshal(temp.Value, &v)
		if err == nil {
			a.Value, err = strconv.ParseInt(v, 10, 64)
		}
	case ApplicationCommandOptionSubCommand:
	case ApplicationCommandOptionSubCommandGroup:
	}

	return err
}

type InteractionApplicationCommandCallbackData struct {
	TTS             bool             `json:"tts,omitempty"`              //	is the response TTS
	Content         *string          `json:"content,omitempty"`          //	message content
	Embeds          []MessageEmbed   `json:"embeds,omitempty"`           // supports up to 10 embeds
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"` // allowed mentions object
	Flags           int              `json:"flags,omitempty"`            //	set to 64 to make your response ephemeral
}

type ThreadMetadata struct {
	Archived            bool                `json:"archived"`              // whether the thread is archived
	AutoArchiveDuration AutoArchiveDuration `json:"auto_archive_duration"` // duration in minutes to automatically archive the thread after recent activity, can be set to: 60, 1440, 4320, 10080
	ArchiveTimestamp    string              `json:"archive_timestamp"`     // timestamp when the thread's archive status was last changed, used for calculating recent activity
	Locked              bool                `json:"locked"`                // whether the thread is locked; when a thread is locked, only users with MANAGE_THREADS can unarchive it
	Invitable           bool                `json:"invitable"`             // Whether non-moderators can add other non-moderators to a thread; only available on private threads
}

// A thread member is used to indicate whether a user has joined a thread or not.
type ThreadMember struct {
	ID            int64     `json:"id,string"`        // the id of the thread (NOT INCLUDED IN GUILDCREATE)
	UserID        int64     `json:"user_id,string"`   // the id of the user (NOT INCLUDED IN GUILDCREATE)
	JoinTimestamp Timestamp `json:"join_timestamp"`   // the time the current user last joined the thread
	Flags         int       `json:"flags"`            // any user-thread settings, currently only used for notifications
	Member        *Member   `json:"member,omitempty"` // Additional information about the user. NOTE: only present if the withMember parameter is set to true when calling Session.ThreadMembers or Session.ThreadMember.
}

// AutoModerationRule stores data for an auto moderation rule.
type AutoModerationRule struct {
	ID              int64                          `json:"id,string,omitempty"`
	GuildID         int64                          `json:"guild_id,string,omitempty"`
	Name            string                         `json:"name,omitempty"`
	CreatorID       int64                          `json:"creator_id,string,omitempty"`
	EventType       AutoModerationRuleEventType    `json:"event_type,omitempty"`
	TriggerType     AutoModerationRuleTriggerType  `json:"trigger_type,omitempty"`
	TriggerMetadata *AutoModerationTriggerMetadata `json:"trigger_metadata,omitempty"`
	Actions         []AutoModerationAction         `json:"actions,omitempty"`
	Enabled         *bool                          `json:"enabled,omitempty"`
	ExemptRoles     IDSlice                        `json:"exempt_roles,omitempty"`
	ExemptChannels  IDSlice                        `json:"exempt_channels,omitempty"`
}

// AutoModerationRuleEventType indicates in what event context a rule should be checked.
type AutoModerationRuleEventType int

// Auto moderation rule event types.
const (
	// AutoModerationEventMessageSend is checked when a member sends or edits a message in the guild
	AutoModerationEventMessageSend AutoModerationRuleEventType = 1
)

// AutoModerationRuleTriggerType represents the type of content which can trigger the rule.
type AutoModerationRuleTriggerType int

// Auto moderation rule trigger types.
const (
	AutoModerationEventTriggerKeyword       AutoModerationRuleTriggerType = 1
	AutoModerationEventTriggerHarmfulLink   AutoModerationRuleTriggerType = 2
	AutoModerationEventTriggerSpam          AutoModerationRuleTriggerType = 3
	AutoModerationEventTriggerKeywordPreset AutoModerationRuleTriggerType = 4
)

// AutoModerationKeywordPreset represents an internally pre-defined wordset.
type AutoModerationKeywordPreset uint

// Auto moderation keyword presets.
const (
	AutoModerationKeywordPresetProfanity     AutoModerationKeywordPreset = 1
	AutoModerationKeywordPresetSexualContent AutoModerationKeywordPreset = 2
	AutoModerationKeywordPresetSlurs         AutoModerationKeywordPreset = 3
)

// AutoModerationTriggerMetadata represents additional metadata used to determine whether rule should be triggered.
type AutoModerationTriggerMetadata struct {
	// Substrings which will be searched for in content.
	// NOTE: should be only used with keyword trigger type.
	KeywordFilter []string `json:"keyword_filter,omitempty"`
	// Regular expression patterns which will be matched against content (maximum of 10).
	// NOTE: should be only used with keyword trigger type.
	RegexPatterns []string `json:"regex_patterns,omitempty"`

	// Internally pre-defined wordsets which will be searched for in content.
	// NOTE: should be only used with keyword preset trigger type.
	Presets []AutoModerationKeywordPreset `json:"presets,omitempty"`

	// Substrings which should not trigger the rule.
	// NOTE: should be only used with keyword or keyword preset trigger type.
	AllowList *[]string `json:"allow_list,omitempty"`

	// Total number of unique role and user mentions allowed per message.
	// NOTE: should be only used with mention spam trigger type.
	MentionTotalLimit int `json:"mention_total_limit,omitempty"`
}

// AutoModerationActionType represents an action which will execute whenever a rule is triggered.
type AutoModerationActionType int

// Auto moderation actions types.
const (
	AutoModerationRuleActionBlockMessage     AutoModerationActionType = 1
	AutoModerationRuleActionSendAlertMessage AutoModerationActionType = 2
	AutoModerationRuleActionTimeout          AutoModerationActionType = 3
)

// AutoModerationActionMetadata represents additional metadata needed during execution for a specific action type.
type AutoModerationActionMetadata struct {
	// Channel to which user content should be logged.
	// NOTE: should be only used with send alert message action type.
	ChannelID int64 `json:"channel_id,string,omitempty"`

	// Timeout duration in seconds (maximum of 2419200 - 4 weeks).
	// NOTE: should be only used with timeout action type.
	Duration int `json:"duration_seconds,omitempty"`
}

// AutoModerationAction stores data for an auto moderation action.
type AutoModerationAction struct {
	Type     AutoModerationActionType      `json:"type"`
	Metadata *AutoModerationActionMetadata `json:"metadata,omitempty"`
}

type SKUType int

// Valid SKUType values
const (
	SKUTypeDurable           SKUType = 2
	SKUTypeConsumable        SKUType = 3
	SKUTypeSubscription      SKUType = 5
	SKUTypeSubscriptionGroup SKUType = 6
)

type SKUFlags int

const (
	// SKUFlagAvailable indicates that the SKU is available for purchase.
	SKUFlagAvailable SKUFlags = 1 << 2
	// SKUFlagGuildSubscription indicates that the SKU is a guild subscription.
	SKUFlagGuildSubscription SKUFlags = 1 << 7
	// SKUFlagUserSubscription indicates that the SKU is a user subscription.
	SKUFlagUserSubscription SKUFlags = 1 << 8
)

// SKU represents a purchasable item in the Discord store.
type SKU struct {
	// The ID of the SKU
	ID int64 `json:"id,string"`

	// The Type of the SKU
	Type SKUType `json:"type"`

	// The ID of the parent application
	ApplicationID int64 `json:"application_id,string"`

	// Customer-facing name of the SKU.
	Name string `json:"name"`

	// System-generated URL slug based on the SKU's name.
	Slug string `json:"slug"`

	// SKUFlags combined as a bitfield. The presence of a certain flag can be checked
	// by performing a bitwise AND operation between this int and the flag.
	Flags SKUFlags `json:"flags"`
}

// EntitlementType is the type of entitlement
// https://discord.com/developers/docs/monetization/entitlements#entitlement-object-entitlement-types
type EntitlementType int

// Valid EntitlementType values
const (
	EntitlementTypePurchase                = 1
	EntitlementTypePremiumSubscription     = 2
	EntitlementTypeDeveloperGift           = 3
	EntitlementTypeTestModePurchase        = 4
	EntitlementTypeFreePurchase            = 5
	EntitlementTypeUserGift                = 6
	EntitlementTypePremiumPurchase         = 7
	EntitlementTypeApplicationSubscription = 8
)

// Entitlement represents that a user or guild has access to a premium offering
// in your application.
type Entitlement struct {
	// The ID of the entitlement
	ID int64 `json:"id,string"`

	// The ID of the SKU
	SKUID int64 `json:"sku_id,string"`

	// The ID of the parent application
	ApplicationID int64 `json:"application_id,string"`

	// The ID of the user that is granted access to the entitlement's sku
	// Only available for user subscriptions.
	UserID int64 `json:"user_id,string,omitempty"`

	// The type of the entitlement
	Type EntitlementType `json:"type"`

	// The entitlement was deleted
	Deleted bool `json:"deleted"`

	// The start date at which the entitlement is valid.
	// Not present when using test entitlements.
	StartsAt *time.Time `json:"starts_at,omitempty"`

	// The date at which the entitlement is no longer valid.
	// Not present when using test entitlements.
	EndsAt *time.Time `json:"ends_at,omitempty"`

	// The ID of the guild that is granted access to the entitlement's sku.
	// Only available for guild subscriptions.
	GuildID int64 `json:"guild_id,string,omitempty"`

	// Whether or not the entitlement has been consumed.
	// Only available for consumable items.
	Consumed bool `json:"consumed,omitempty"`
}

// EntitlementOwnerType is the type of entitlement (see EntitlementOwnerType* consts)
type EntitlementOwnerType int

// Valid EntitlementOwnerType values
const (
	EntitlementOwnerTypeGuildSubscription EntitlementOwnerType = 1
	EntitlementOwnerTypeUserSubscription  EntitlementOwnerType = 2
)

// EntitlementTest is used to test granting an entitlement to a user or guild
type EntitlementTest struct {
	// The ID of the SKU to grant the entitlement to
	SKUID int64 `json:"sku_id,string"`

	// The ID of the guild or user to grant the entitlement to
	OwnerID int64 `json:"owner_id,string"`

	// OwnerType is the type of which the entitlement should be created
	OwnerType EntitlementOwnerType `json:"owner_type"`
}

// EntitlementFilterOptions are the options for filtering Entitlements
type EntitlementFilterOptions struct {
	// Optional user ID to look up for.
	UserID int64

	// Optional array of SKU IDs to check for.
	SkuIDs []int64

	// Optional timestamp (snowflake) to retrieve Entitlements before this time.
	BeforeID int64

	// Optional timestamp (snowflake) to retrieve Entitlements after this time.
	AfterID int64

	// Optional maximum number of entitlements to return (1-100, default 100).
	Limit int

	// Optional guild ID to look up for.
	GuildID int64

	// Optional whether or not ended entitlements should be omitted.
	ExcludeEnded bool
}

// Tells the status of a subscription
type SubscriptionStatus int

const (
	//Subscription is Active and scheduled to renew
	SubscriptionStatusActive SubscriptionStatus = 1
	//Subscription is Active but scheduled to end and not be renewed
	SubscriptionStatusEnding SubscriptionStatus = 2
	//Subscription is Inactive and not being charged
	SubscriptionStatusInactive SubscriptionStatus = 3
)

type Subscription struct {
	ID     int64 `json:"id,string"`
	UserID int64 `json:"user_id,string"`
	//list of SKUs the user has subscribed to
	SKUIDS []int64 `json:"sku_ids"`
	//list of entitlements granted
	EntitlementIDs []int64 `json:"entitlement_ids"`
	//start of the current subscription period, this is updated when the subscription is renewed, not to be confused with the start of a subscription
	CurrentPeriodStart time.Time `json:"current_period_start"`
	//end of the current subscription period
	CurrentPeriodEnd time.Time          `json:"current_period_end"`
	Status           SubscriptionStatus `json:"status"`
	// Only present if the subscription status is Ending or Inactive, represents the time when the subscription was cancelled
	CancelledAt time.Time `json:"cancelled_at,omitempty"`
}

// SubscriptionFilterOptions are the options for filtering Subscriptions for list subscriptions API
type SubscriptionFilterOptions struct {
	BeforeID int64
	AfterID  int64
	Limit    int
	UserId   int64
}

type CurrentGuildMemberUpdate struct {
	Nick   string `json:"nick,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	Banner string `json:"banner,omitempty"`
}
