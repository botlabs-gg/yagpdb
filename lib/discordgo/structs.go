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
	"github.com/volatiletech/null"
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
	return t == ChannelTypeGuildPrivateThread || t == ChannelTypeGuildPublicThread
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

	RateLimitPerUser int `json:"rate_limit_per_user"`

	ThreadMetadata *ThreadMetadata `json:"thread_metadata"`
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

// A ChannelEdit holds Channel Feild data for a channel edit.
type ChannelEdit struct {
	Name                 string                 `json:"name,omitempty"`
	Topic                string                 `json:"topic,omitempty"`
	NSFW                 bool                   `json:"nsfw,omitempty"`
	Position             *int                   `json:"position,omitempty"`
	Bitrate              int                    `json:"bitrate,omitempty"`
	UserLimit            int                    `json:"user_limit,omitempty"`
	PermissionOverwrites []*PermissionOverwrite `json:"permission_overwrites,omitempty"`
	ParentID             *null.String           `json:"parent_id,omitempty"`
	RateLimitPerUser     *int                   `json:"rate_limit_per_user,omitempty"`
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

	ApproximateMemberCount   int `json:"approximate_member_count"`
	ApproximatePresenceCount int `json:"approximate_presence_count"`
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
}

// Mention returns a string which mentions the role
func (r *Role) Mention() string {
	return fmt.Sprintf("<@&%d>", r.ID)
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
	UserID    int64  `json:"user_id,string"`
	SessionID string `json:"session_id"`
	ChannelID int64  `json:"channel_id,string"`
	GuildID   int64  `json:"guild_id,string"`
	Suppress  bool   `json:"suppress"`
	SelfMute  bool   `json:"self_mute"`
	SelfDeaf  bool   `json:"self_deaf"`
	Mute      bool   `json:"mute"`
	Deaf      bool   `json:"deaf"`
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

type Activities []*Game

func (a *Activities) UnmarshalJSONArray(dec *gojay.Decoder) error {
	instance := Game{}
	err := dec.Object(&instance)
	if err != nil {
		return err
	}
	*a = append(*a, &instance)
	return nil
}

// GameType is the type of "game" (see GameType* consts) in the Game struct
type GameType int

// Valid GameType values
const (
	GameTypeGame GameType = iota
	GameTypeStreaming
	GameTypeListening
	GameTypeWatching
)

// A Game struct holds the name of the "playing .." game for a user
type Game struct {
	Name          string     `json:"name"`
	Type          GameType   `json:"type"`
	URL           string     `json:"url,omitempty"`
	Details       string     `json:"details,omitempty"`
	State         string     `json:"state,omitempty"`
	TimeStamps    TimeStamps `json:"timestamps,omitempty"`
	Assets        Assets     `json:"assets,omitempty"`
	ApplicationID string     `json:"application_id,omitempty"`
	Instance      int8       `json:"instance,omitempty"`
	// TODO: Party and Secrets (unknown structure)
}

// implement gojay.UnmarshalerJSONObject
func (g *Game) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "name":
		return dec.String(&g.Name)
	case "type":
		return dec.Int((*int)(&g.Type))
	case "url":
		return dec.String(&g.URL)
	case "details":
		return dec.String(&g.Details)
	case "state":
		return dec.String(&g.State)
	case "timestamps":
		return dec.Object(&g.TimeStamps)
	case "assets":
	case "application_id":
		var i interface{}
		err := dec.Interface(&i)
		if err != nil {
			return err
		}
		switch t := i.(type) {
		case int64:
			g.ApplicationID = strconv.FormatInt(t, 10)
		case int32:
			g.ApplicationID = strconv.FormatInt(int64(t), 10)
		case string:
			g.ApplicationID = t
		}
	case "instance":
		return dec.Int8(&g.Instance)
	}

	return nil
}

func (g *Game) NKeys() int {
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

	// Whether the member is deafened at a guild level.
	Deaf bool `json:"deaf"`

	// Whether the member is muted at a guild level.
	Mute bool `json:"mute"`

	// The underlying user on which the member is based.
	User *User `json:"user"`

	// A list of IDs of the roles which are possessed by the member.
	Roles IDSlice `json:"roles,string"`

	// Whether the user has not yet passed the guild's Membership Screening requirements
	Pending bool `json:"pending"`

	// The time at which the member's timeout will expire.
	// Time in the past or nil if the user is not timed out.
	TimeoutExpiresAt *time.Time `json:"communication_disabled_until"`
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

type AuditLogEntry struct {
	TargetID int64 `json:"target_id,string"`
	Changes  []struct {
		NewValue interface{} `json:"new_value"`
		OldValue interface{} `json:"old_value"`
		Key      string      `json:"key"`
	} `json:"changes,omitempty"`
	UserID     int64 `json:"user_id,string"`
	ID         int64 `json:"id,string"`
	ActionType int   `json:"action_type"`
	Options    struct {
		DeleteMembersDay string `json:"delete_member_days"`
		MembersRemoved   string `json:"members_removed"`
		ChannelID        int64  `json:"channel_id,string"`
		Count            string `json:"count"`
		ID               int64  `json:"id,string"`
		Type             string `json:"type"`
		RoleName         string `json:"role_name"`
	} `json:"options,omitempty"`
	Reason string `json:"reason"`
}

// Block contains Discord Audit Log Action Types
const (
	AuditLogActionGuildUpdate = 1

	AuditLogActionChannelCreate          = 10
	AuditLogActionChannelUpdate          = 11
	AuditLogActionChannelDelete          = 12
	AuditLogActionChannelOverwriteCreate = 13
	AuditLogActionChannelOverwriteUpdate = 14
	AuditLogActionChannelOverwriteDelete = 15

	AuditLogActionMemberKick       = 20
	AuditLogActionMemberPrune      = 21
	AuditLogActionMemberBanAdd     = 22
	AuditLogActionMemberBanRemove  = 23
	AuditLogActionMemberUpdate     = 24
	AuditLogActionMemberRoleUpdate = 25

	AuditLogActionRoleCreate = 30
	AuditLogActionRoleUpdate = 31
	AuditLogActionRoleDelete = 32

	AuditLogActionInviteCreate = 40
	AuditLogActionInviteUpdate = 41
	AuditLogActionInviteDelete = 42

	AuditLogActionWebhookCreate = 50
	AuditLogActionWebhookUpdate = 51
	AuditLogActionWebhookDelete = 52

	AuditLogActionEmojiCreate = 60
	AuditLogActionEmojiUpdate = 61
	AuditLogActionEmojiDelete = 62

	AuditLogActionMessageDelete = 72
)

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
	Content         string             `json:"content,omitempty"`
	Username        string             `json:"username,omitempty"`
	AvatarURL       string             `json:"avatar_url,omitempty"`
	TTS             bool               `json:"tts,omitempty"`
	File            *File              `json:"-, omitempty"`
	Components      []MessageComponent `json:"components"`
	Embeds          []*MessageEmbed    `json:"embeds,omitempty"`
	Flags           int64              `json:"flags,omitempty"`
	AllowedMentions *AllowedMentions   `json:"allowed_mentions,omitempty"`
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
	Total      int   `json:"total"`
	Remaining  int   `json:"remaining"`
	ResetAfter int64 `json:"reset_after"`
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

	ErrCodeUnauthorized = 40001

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
}

type interactionTemp struct {
	ID            int64           `json:"id,string"`             // id of the interaction
	ApplicationID int64           `json:"application_id,string"` // id of the application this interaction is for
	Kind          InteractionType `json:"type"`                  // the type of interaction
	Data          json.RawMessage `json:"data"`                  // data payload
	GuildID       int64           `json:"guild_id,string"`       // the guild it was sent from
	ChannelID     int64           `json:"channel_id,string"`     // the channel it was sent from
	Member        *Member         `json:"member"`                // member object	guild member data for the invoking user, including permissions
	User          *User           `json:"user"`                  // object	user object for the invoking user, if invoked in a DM
	Token         string          `json:"token"`                 // a continuation token for responding to the interaction
	Version       int             `json:"version"`               // read-only property, always
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
	Archived            bool   `json:"archived"`              // whether the thread is archived
	AutoArchiveDuration int    `json:"auto_archive_duration"` // duration in minutes to automatically archive the thread after recent activity, can be set to: 60, 1440, 4320, 10080
	ArchiveTimestamp    string `json:"archive_timestamp"`     // timestamp when the thread's archive status was last changed, used for calculating recent activity
	Locked              bool   `json:"locked"`                // whether the thread is locked; when a thread is locked, only users with MANAGE_THREADS can unarchive it
}

// A thread member is used to indicate whether a user has joined a thread or not.
type ThreadMember struct {
	ID            int64     `json:"id,string"`      // the id of the thread (NOT INCLUDED IN GUILDCREATE)
	UserID        int64     `json:"user_id,string"` // the id of the user (NOT INCLUDED IN GUILDCREATE)
	JoinTimestamp Timestamp `json:"join_timestamp"` // the time the current user last joined the thread
	Flags         int       `json:"flags"`          // any user-thread settings, currently only used for notifications
}
