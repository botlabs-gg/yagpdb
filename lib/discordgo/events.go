package discordgo

import (
	"encoding/json"

	"github.com/botlabs-gg/yagpdb/v2/lib/gojay"
	"github.com/pkg/errors"
)

// This file contains all the possible structs that can be
// handled by AddHandler/EventHandler.
// DO NOT ADD ANYTHING BUT EVENT HANDLER STRUCTS TO THIS FILE.
//go:generate go run tools/cmd/eventhandlers/main.go

// Connect is the data for a Connect event.
// This is a sythetic event and is not dispatched by Discord.
type Connect struct{}

// Disconnect is the data for a Disconnect event.
// This is a sythetic event and is not dispatched by Discord.
type Disconnect struct{}

// RateLimit is the data for a RateLimit event.
// This is a sythetic event and is not dispatched by Discord.
type RateLimit struct {
	*TooManyRequests
	URL string
}

// Event provides a basic initial struct for all websocket events.
type Event struct {
	Operation GatewayOP          `json:"op"`
	Sequence  int64              `json:"s"`
	Type      string             `json:"t"`
	RawData   gojay.EmbeddedJSON `json:"d"`
	// Struct contains one of the other types in this file.
	Struct interface{} `json:"-"`
}

// implement gojay.UnmarshalerJSONObject
func (evt *Event) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "op":
		return dec.Int((*int)(&evt.Operation))
	case "s":
		return dec.Int64(&evt.Sequence)
	case "t":
		return dec.String(&evt.Type)
	case "d":
		if cap(evt.RawData) > 1000000 && len(evt.RawData) < 100000 {
			evt.RawData = nil
		} else if evt.RawData != nil {
			evt.RawData = evt.RawData[:0]
		}

		return dec.AddEmbeddedJSON(&evt.RawData)
	}

	return nil
}

func (evt *Event) NKeys() int {
	return 0
}

// A Ready stores all data for the websocket READY event.
type Ready struct {
	Version          int          `json:"v"`
	SessionID        string       `json:"session_id"`
	User             *SelfUser    `json:"user"`
	ReadState        []*ReadState `json:"read_state"`
	PrivateChannels  []*Channel   `json:"private_channels"`
	Guilds           []*Guild     `json:"guilds"`
	ResumeGatewayUrl string       `json:"resume_gateway_url"`

	// Undocumented fields
	Settings          *Settings            `json:"user_settings"`
	UserGuildSettings []*UserGuildSettings `json:"user_guild_settings"`
	Relationships     []*Relationship      `json:"relationships"`
	Presences         []*Presence          `json:"presences"`
	Notes             map[string]string    `json:"notes"`
}

// ChannelCreate is the data for a ChannelCreate event.
type ChannelCreate struct {
	*Channel
}

// ChannelUpdate is the data for a ChannelUpdate event.
type ChannelUpdate struct {
	*Channel
}

// ChannelDelete is the data for a ChannelDelete event.
type ChannelDelete struct {
	*Channel
}

// ChannelPinsUpdate stores data for a ChannelPinsUpdate event.
type ChannelPinsUpdate struct {
	LastPinTimestamp string `json:"last_pin_timestamp"`
	ChannelID        int64  `json:"channel_id,string"`
	GuildID          int64  `json:"guild_id,string,omitempty"`
}

func (cp *ChannelPinsUpdate) GetGuildID() int64 {
	return cp.GuildID
}

func (cp *ChannelPinsUpdate) GetChannelID() int64 {
	return cp.ChannelID
}

// GuildCreate is the data for a GuildCreate event.
type GuildCreate struct {
	*Guild
}

// GuildUpdate is the data for a GuildUpdate event.
type GuildUpdate struct {
	*Guild
}

// GuildDelete is the data for a GuildDelete event.
type GuildDelete struct {
	*Guild
}

// GuildBanAdd is the data for a GuildBanAdd event.
type GuildBanAdd struct {
	User    *User `json:"user"`
	GuildID int64 `json:"guild_id,string"`
}

func (gba *GuildBanAdd) GetGuildID() int64 {
	return gba.GuildID
}

// GuildBanRemove is the data for a GuildBanRemove event.
type GuildBanRemove struct {
	User    *User `json:"user"`
	GuildID int64 `json:"guild_id,string"`
}

func (e *GuildBanRemove) GetGuildID() int64 {
	return e.GuildID
}

// GuildMemberAdd is the data for a GuildMemberAdd event.
type GuildMemberAdd struct {
	*Member
}

func (e *GuildMemberAdd) GetGuildID() int64 {
	return e.GuildID
}

// GuildMemberUpdate is the data for a GuildMemberUpdate event.
type GuildMemberUpdate struct {
	*Member
}

// GuildMemberRemove is the data for a GuildMemberRemove event.
type GuildMemberRemove struct {
	*Member
}

// GuildRoleCreate is the data for a GuildRoleCreate event.
type GuildRoleCreate struct {
	*GuildRole
}

// GuildRoleUpdate is the data for a GuildRoleUpdate event.
type GuildRoleUpdate struct {
	*GuildRole
}

// A GuildRoleDelete is the data for a GuildRoleDelete event.
type GuildRoleDelete struct {
	RoleID  int64 `json:"role_id,string"`
	GuildID int64 `json:"guild_id,string"`
}

func (e *GuildRoleDelete) GetGuildID() int64 {
	return e.GuildID
}

// A GuildEmojisUpdate is the data for a guild emoji update event.
type GuildEmojisUpdate struct {
	GuildID int64    `json:"guild_id,string"`
	Emojis  []*Emoji `json:"emojis"`
}

func (e *GuildEmojisUpdate) GetGuildID() int64 {
	return e.GuildID
}

// A GuildMembersChunk is the data for a GuildMembersChunk event.
type GuildMembersChunk struct {
	GuildID    int64     `json:"guild_id,string"`
	Members    []*Member `json:"members"`
	ChunkIndex int       `json:"chunk_index"`
	ChunkCount int       `json:"chunk_count"`
	Nonce      string    `json:"nonce"`
}

func (e *GuildMembersChunk) GetGuildID() int64 {
	return e.GuildID
}

// GuildIntegrationsUpdate is the data for a GuildIntegrationsUpdate event.
type GuildIntegrationsUpdate struct {
	GuildID int64 `json:"guild_id,string"`
}

func (e *GuildIntegrationsUpdate) GetGuildID() int64 {
	return e.GuildID
}

// MessageAck is the data for a MessageAck event.
type MessageAck struct {
	MessageID int64 `json:"message_id,string"`
	ChannelID int64 `json:"channel_id,string"`
}

// MessageCreate is the data for a MessageCreate event.
type MessageCreate struct {
	*Message
}

// UnmarshalJSON is a helper function to unmarshal MessageCreate object.
func (m *MessageCreate) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &m.Message)
}

// MessageUpdate is the data for a MessageUpdate event.
type MessageUpdate struct {
	*Message
}

// UnmarshalJSON is a helper function to unmarshal MessageUpdate object.
func (m *MessageUpdate) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &m.Message)
}

// MessageDelete is the data for a MessageDelete event.
type MessageDelete struct {
	*Message
}

// UnmarshalJSON is a helper function to unmarshal MessageDelete object.
func (m *MessageDelete) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &m.Message)
}

// MessageReactionAdd is the data for a MessageReactionAdd event.
type MessageReactionAdd struct {
	*MessageReaction
}

// MessageReactionRemove is the data for a MessageReactionRemove event.
type MessageReactionRemove struct {
	*MessageReaction
}

// MessageReactionRemoveAll is the data for a MessageReactionRemoveAll event.
type MessageReactionRemoveAll struct {
	*MessageReaction
}

// all reactions for a given emoji were explicitly removed from a message
type MessageReactionRemoveEmoji struct {
	ChannelID int64 `json:"channel_id,string"`
	GuildID   int64 `json:"guild_id,string"`
	MessageID int64 `json:"message_id,string"`
	Emoji     Emoji `json:"emoji"`
}

// PresencesReplace is the data for a PresencesReplace event.
type PresencesReplace []*Presence

// PresenceUpdate is the data for a PresenceUpdate event.
type PresenceUpdate struct {
	Presence
	GuildID int64 `json:"guild_id,string"`
}

func (e *PresenceUpdate) GetGuildID() int64 {
	return e.GuildID
}

// implement gojay.UnmarshalerJSONObject
func (p *PresenceUpdate) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "guild_id":
		return errors.Wrap(DecodeSnowflake(&p.GuildID, dec), key)
	default:
		return p.Presence.UnmarshalJSONObject(dec, key)
	}
}

func (p *PresenceUpdate) NKeys() int {
	return 0
}

// Resumed is the data for a Resumed event.
type Resumed struct {
	Trace []string `json:"_trace"`
}

// RelationshipAdd is the data for a RelationshipAdd event.
type RelationshipAdd struct {
	*Relationship
}

// RelationshipRemove is the data for a RelationshipRemove event.
type RelationshipRemove struct {
	*Relationship
}

var _ gojay.UnmarshalerJSONObject = (*TypingStart)(nil)

// TypingStart is the data for a TypingStart event.
type TypingStart struct {
	UserID    int64 `json:"user_id,string"`
	ChannelID int64 `json:"channel_id,string"`
	Timestamp int   `json:"timestamp"`
	GuildID   int64 `json:"guild_id,string,omitempty"`
}

// implement gojay.UnmarshalerJSONObject
func (ts *TypingStart) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "user_id":
		return DecodeSnowflake(&ts.UserID, dec)
	case "channel_id":
		return DecodeSnowflake(&ts.ChannelID, dec)
	case "guild_id":
		return DecodeSnowflake(&ts.GuildID, dec)
	case "timestamp":
		return dec.Int(&ts.Timestamp)
	}

	return nil
}

func (ts *TypingStart) NKeys() int {
	return 0
}

func (e *TypingStart) GetGuildID() int64 {
	return e.GuildID
}

func (e *TypingStart) GetChannelID() int64 {
	return e.ChannelID
}

// UserUpdate is the data for a UserUpdate event.
type UserUpdate struct {
	*User
}

// implement gojay.UnmarshalerJSONObject
func (u *UserUpdate) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	u.User = &User{}
	return u.User.UnmarshalJSONObject(dec, key)
}

func (u *UserUpdate) NKeys() int {
	return 0
}

// UserSettingsUpdate is the data for a UserSettingsUpdate event.
type UserSettingsUpdate map[string]interface{}

// UserGuildSettingsUpdate is the data for a UserGuildSettingsUpdate event.
type UserGuildSettingsUpdate struct {
	*UserGuildSettings
}

// UserNoteUpdate is the data for a UserNoteUpdate event.
type UserNoteUpdate struct {
	ID   int64  `json:"id,string"`
	Note string `json:"note"`
}

// VoiceServerUpdate is the data for a VoiceServerUpdate event.
type VoiceServerUpdate struct {
	Token    string `json:"token"`
	GuildID  int64  `json:"guild_id,string"`
	Endpoint string `json:"endpoint"`
}

func (e *VoiceServerUpdate) GetGuildID() int64 {
	return e.GuildID
}

// VoiceStateUpdate is the data for a VoiceStateUpdate event.
type VoiceStateUpdate struct {
	*VoiceState
}

// MessageDeleteBulk is the data for a MessageDeleteBulk event
type MessageDeleteBulk struct {
	Messages  IDSlice `json:"ids"`
	ChannelID int64   `json:"channel_id,string"`
	GuildID   int64   `json:"guild_id,string"`
}

func (e *MessageDeleteBulk) GetGuildID() int64 {
	return e.GuildID
}

func (e *MessageDeleteBulk) GetChannelID() int64 {
	return e.ChannelID
}

// WebhooksUpdate is the data for a WebhooksUpdate event
type WebhooksUpdate struct {
	GuildID   int64 `json:"guild_id,string"`
	ChannelID int64 `json:"channel_id,string"`
}

func (e *WebhooksUpdate) GetGuildID() int64 {
	return e.GuildID
}

func (e *WebhooksUpdate) GetChannelID() int64 {
	return e.ChannelID
}

// InviteCreate is the data for the InviteCreate event
type InviteCreate struct {
	GuildID   int64 `json:"guild_id,string"`
	ChannelID int64 `json:"channel_id,string"`

	Code      string    `json:"code"`
	CreatedAt Timestamp `json:"created_at"`

	MaxAge    int  `json:"max_age"`
	MaxUses   int  `json:"max_uses"`
	Temporary bool `json:"temporary"`
	Uses      int  `json:"uses"`

	Inviter *InviteUser `json:"inviter"`
}

// InviteDelete is the data for the InviteDelete event
type InviteDelete struct {
	GuildID   int64  `json:"guild_id,string"`
	ChannelID int64  `json:"channel_id,string"`
	Code      string `json:"code"`
}

type InteractionCreate struct {
	Interaction
}

// new Slash Command was created
type ApplicationCommandCreate struct {
	GuildID int64 `json:"guild_id,string"`
	ApplicationCommand
}

// Slash Command was updated
type ApplicationCommandUpdate struct {
	GuildID int64 `json:"guild_id,string"`
	ApplicationCommand
}

// Slash Command was deleted
type ApplicationCommandDelete struct {
	GuildID int64 `json:"guild_id,string"`
	ApplicationCommand
}

// thread created, also sent when being added to a private thread
type ThreadCreate struct {
	Channel
}

// thread was updated
type ThreadUpdate struct {
	Channel
}

// thread was deleted
type ThreadDelete struct {
	ID       int64 `json:"id,string"`
	GuildID  int64 `json:"guild_id,string"`
	ParentID int64 `json:"parent_id,string"`
	Type     ChannelType
}

// sent when gaining access to a channel, contains all active threads in that channel
type ThreadListSync struct {
	GuildID  int64           `json:"guild_id,string"` // snowflake	the id of the guild
	Channels IDSlice         `json:"channel_ids"`     // array of snowflakes	the parent channel ids whose threads are being synced. If omitted, then threads were synced for the entire guild. This array may contain channel_ids that have no active threads as well, so you know to clear that data.
	Threads  []*Channel      `json:"threads"`         // array of channel objects	all active threads in the given channels that the current user can access
	Members  []*ThreadMember `json:"members"`         // array of thread member objects	all thread member objects from the synced threads for the current user, indicating which threads the current user has been added to
}

// thread member for the current user was updated
type ThreadMemberUpdate struct {
	*ThreadMember
	GuildID int64 `json:"guild_id,string"` // snowflake	the id of the guild
}

// some user(s) were added to or removed from a thread
type ThreadMembersUpdate struct {
	ID             int64           `json:"id,string"`          // snowflake	the id of the thread
	GuildID        int64           `json:"guild_id,string"`    // snowflake	the id of the guild
	MemberCount    int             `json:"member_count"`       // integer	the approximate number of members in the thread, capped at 50
	AddedMembers   []*ThreadMember `json:"added_members"`      // array of thread member objects	the users who were added to the thread
	RemovedMembers IDSlice         `json:"removed_member_ids"` // array of snowflakes	the id of the users who were removed from the thread
}

// guild stickers were updated
type GuildStickersUpdate struct {
	GuildID  int64      `json:"guild_id,string"`
	Stickers []*Sticker `json:"stickers"`
}

func (e *GuildStickersUpdate) GetGuildID() int64 {
	return e.GuildID
}

// stage instance was created
type StageInstanceCreate struct {
}

// stage instance was deleted or closed
type StageInstanceDelete struct {
}

// stage instance was updated
type StageInstanceUpdate struct {
}

// ApplicationCommandPermissionsUpdate is the data for an ApplicationCommandPermissionsUpdate event
type ApplicationCommandPermissionsUpdate struct {
	*GuildApplicationCommandPermissions
}

// AutoModerationRuleCreate is the data for an AutoModerationRuleCreate event.
type AutoModerationRuleCreate struct {
	*AutoModerationRule
}

// AutoModerationRuleUpdate is the data for an AutoModerationRuleUpdate event.
type AutoModerationRuleUpdate struct {
	*AutoModerationRule
}

// AutoModerationRuleDelete is the data for an AutoModerationRuleDelete event.
type AutoModerationRuleDelete struct {
	*AutoModerationRule
}

// AutoModerationActionExecution is the data for an AutoModerationActionExecution event.
type AutoModerationActionExecution struct {
	GuildID              int64                         `json:"guild_id,string"`
	Action               AutoModerationAction          `json:"action"`
	RuleID               int64                         `json:"rule_id,string"`
	RuleTriggerType      AutoModerationRuleTriggerType `json:"rule_trigger_type"`
	UserID               int64                         `json:"user_id,string"`
	ChannelID            int64                         `json:"channel_id,string"`
	MessageID            int64                         `json:"message_id,string"`
	AlertSystemMessageID int64                         `json:"alert_system_message_id,string"`
	Content              string                        `json:"content"`
	MatchedKeyword       string                        `json:"matched_keyword"`
	MatchedContent       string                        `json:"matched_content"`
}

func (e *AutoModerationActionExecution) GetGuildID() int64 {
	return e.GuildID
}

type GuildAuditLogEntryCreate struct {
	*AuditLogEntry
}

type GuildJoinRequestUpdate struct{}
type GuildJoinRequestDelete struct{}
type VoiceChannelStatusUpdate struct{}
type ChannelTopicUpdate struct{}

// Monetization events
type EntitlementCreate struct {
	*Entitlement
}
type EntitlementUpdate struct {
	*Entitlement
}

// EntitlementDelete is the data for an EntitlementDelete event.
// NOTE: Entitlements are not deleted when they expire.
type EntitlementDelete struct {
	*Entitlement
}

type SubscriptionCreate struct {
	*Subscription
}
type SubscriptionUpdate struct {
	*Subscription
}
type SubscriptionDelete struct {
	*Subscription
}
