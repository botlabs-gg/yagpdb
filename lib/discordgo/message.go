// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains code related to the Message struct

package discordgo

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// MessageType is the type of Message
type MessageType int

// Block contains the valid known MessageType values
const (
	MessageTypeDefault                                 MessageType = 0
	MessageTypeRecipientAdd                            MessageType = 1
	MessageTypeRecipientRemove                         MessageType = 2
	MessageTypeCall                                    MessageType = 3
	MessageTypeChannelNameChange                       MessageType = 4
	MessageTypeChannelIconChange                       MessageType = 5
	MessageTypeChannelPinnedMessage                    MessageType = 6
	MessageTypeGuildMemberJoin                         MessageType = 7
	MessageTypeUserPremiumGuildSubscription            MessageType = 8
	MessageTypeUserPremiumGuildSubscriptionTier1       MessageType = 9
	MessageTypeUserPremiumGuildSubscriptionTier2       MessageType = 10
	MessageTypeUserPremiumGuildSubscriptionTier3       MessageType = 11
	MessageTypeChannelFollowAdd                        MessageType = 12
	MessageTypeGuildDiscoveryDisqualified              MessageType = 14
	MessageTypeGuildDiscoveryRequalified               MessageType = 15
	MessageTypeGuildDiscoveryGracePeriodInitialWarning MessageType = 16
	MessageTypeGuildDiscoveryGracePeriodFinalWarning   MessageType = 17
	MessageTypeThreadCreated                           MessageType = 18
	MessageTypeReply                                   MessageType = 19
	MessageTypeApplicationCommand                      MessageType = 20
	MessageTypeThreadStarterMessage                    MessageType = 21
	MessageTypeGuildInviteReminder                     MessageType = 22
)

// IsSystem returns wether the message type is a system message type, a message created by discord
func (m MessageType) IsSystem() bool {
	switch m {
	case MessageTypeRecipientAdd,
		MessageTypeRecipientRemove,
		MessageTypeCall,
		MessageTypeChannelNameChange,
		MessageTypeChannelIconChange,
		MessageTypeChannelPinnedMessage,
		MessageTypeGuildMemberJoin,
		MessageTypeUserPremiumGuildSubscription,
		MessageTypeUserPremiumGuildSubscriptionTier1,
		MessageTypeUserPremiumGuildSubscriptionTier2,
		MessageTypeUserPremiumGuildSubscriptionTier3,
		MessageTypeChannelFollowAdd,
		MessageTypeGuildDiscoveryDisqualified,
		MessageTypeGuildDiscoveryRequalified,
		MessageTypeGuildDiscoveryGracePeriodInitialWarning,
		MessageTypeGuildDiscoveryGracePeriodFinalWarning,
		MessageTypeThreadCreated,
		MessageTypeGuildInviteReminder:
		return true
	default:
		return false
	}
}

// A Message stores all data related to a specific Discord message.
type Message struct {
	// The ID of the message.
	ID int64 `json:"id,string"`

	// The ID of the channel in which the message was sent.
	ChannelID int64 `json:"channel_id,string"`

	// The ID of the guild in which the message was sent.
	GuildID int64 `json:"guild_id,string,omitempty"`

	// The content of the message.
	Content string `json:"content"`

	// The time at which the messsage was sent.
	// CAUTION: this field may be removed in a
	// future API version; it is safer to calculate
	// the creation time via the ID.
	Timestamp Timestamp `json:"timestamp"`

	// The time at which the last edit of the message
	// occurred, if it has been edited.
	EditedTimestamp Timestamp `json:"edited_timestamp"`

	// The roles mentioned in the message.
	MentionRoles IDSlice `json:"mention_roles,string"`

	// Whether the message is text-to-speech.
	TTS bool `json:"tts"`

	// Whether the message mentions everyone.
	MentionEveryone bool `json:"mention_everyone"`

	// The author of the message. This is not guaranteed to be a
	// valid user (webhook-sent messages do not possess a full author).
	Author *User `json:"author"`

	// A list of attachments present in the message.
	Attachments []*MessageAttachment `json:"attachments"`

	// A list of components attached to the message.
	Components []MessageComponent `json:"-"`

	// A list of embeds present in the message. Multiple
	// embeds can currently only be sent by webhooks.
	Embeds []*MessageEmbed `json:"embeds"`

	// A list of users mentioned in the message.
	Mentions []*User `json:"mentions"`

	// Whether the message is pinned or not.
	Pinned bool `json:"pinned"`

	// A list of reactions to the message.
	Reactions []*MessageReactions `json:"reactions"`

	// The type of the message.
	Type MessageType `json:"type"`

	WebhookID int64 `json:"webhook_id,string"`

	Member *Member `json:"member"`

	// MessageReference contains reference data sent with crossposted or reply messages.
	// This does not contain the reference *to* this message; this is for when *this* message references another.
	// To generate a reference to this message, use (*Message).Reference().
	MessageReference *MessageReference `json:"message_reference"`

	// The message associated with the message_reference
	// NOTE: This field is only returned for messages with a type of 19 (REPLY) or 21 (THREAD_STARTER_MESSAGE).
	// If the message is a reply but the referenced_message field is not present,
	// the backend did not attempt to fetch the message that was being replied to, so its state is unknown.
	// If the field exists but is null, the referenced message was deleted.
	ReferencedMessage *Message `json:"referenced_message"`

	// Is sent when the message is a response to an Interaction, without an existing message.
	// This means responses to message component interactions do not include this property,
	// instead including a MessageReference, as components exist on preexisting messages.
	Interaction *MessageInteraction `json:"interaction"`
}

func (m *Message) GetGuildID() int64 {
	return m.GuildID
}

func (m *Message) GetChannelID() int64 {
	return m.ChannelID
}

func (m *Message) Link() string {
	return fmt.Sprintf("https://discord.com/channels/%v/%v/%v", m.GuildID, m.ChannelID, m.ID)
}

// MessageFlags is the flags of "message" (see MessageFlags* consts)
// https://discord.com/developers/docs/resources/channel#message-object-message-flags
type MessageFlags int

// Valid MessageFlags values
const (
	// MessageFlagsCrossPosted This message has been published to subscribed channels (via Channel Following).
	MessageFlagsCrossPosted MessageFlags = 1 << 0
	// MessageFlagsIsCrossPosted this message originated from a message in another channel (via Channel Following).
	MessageFlagsIsCrossPosted MessageFlags = 1 << 1
	// MessageFlagsSuppressEmbeds do not include any embeds when serializing this message.
	MessageFlagsSuppressEmbeds MessageFlags = 1 << 2
	// TODO: deprecated, remove when compatibility is not needed
	MessageFlagsSupressEmbeds MessageFlags = 1 << 2
	// MessageFlagsSourceMessageDeleted the source message for this crosspost has been deleted (via Channel Following).
	MessageFlagsSourceMessageDeleted MessageFlags = 1 << 3
	// MessageFlagsUrgent this message came from the urgent message system.
	MessageFlagsUrgent MessageFlags = 1 << 4
	// MessageFlagsHasThread this message has an associated thread, with the same id as the message.
	MessageFlagsHasThread MessageFlags = 1 << 5
	// MessageFlagsEphemeral this message is only visible to the user who invoked the Interaction.
	MessageFlagsEphemeral MessageFlags = 1 << 6
	// MessageFlagsLoading this message is an Interaction Response and the bot is "thinking".
	MessageFlagsLoading MessageFlags = 1 << 7
	// MessageFlagsFailedToMentionSomeRolesInThread this message failed to mention some roles and add their members to the thread.
	MessageFlagsFailedToMentionSomeRolesInThread MessageFlags = 1 << 8
)

// File stores info about files you e.g. send in messages.
type File struct {
	Name        string
	ContentType string
	Reader      io.Reader
}

// MessageSend stores all parameters you can send with ChannelMessageSendComplex.
type MessageSend struct {
	Content         string             `json:"content,omitempty"`
	Embeds          []*MessageEmbed    `json:"embeds,omitempty"`
	TTS             bool               `json:"tts"`
	Components      []MessageComponent `json:"components"`
	Files           []*File            `json:"-"`
	AllowedMentions AllowedMentions    `json:"allowed_mentions,omitempty"`
	Reference       *MessageReference  `json:"message_reference,omitempty"`

	// TODO: Remove this when compatibility is not required.
	File *File `json:"-"`
}

// MessageEdit is used to chain parameters via ChannelMessageEditComplex, which
// is also where you should get the instance from.
type MessageEdit struct {
	Content         *string            `json:"content,omitempty"`
	Components      []MessageComponent `json:"components"`
	Embeds          []*MessageEmbed    `json:"embeds,omitempty"`
	AllowedMentions AllowedMentions    `json:"allowed_mentions,omitempty"`
	Flags           MessageFlags       `json:"flags,omitempty"`

	ID      int64
	Channel int64
}

// NewMessageEdit returns a MessageEdit struct, initialized
// with the Channel and ID.
func NewMessageEdit(channelID int64, messageID int64) *MessageEdit {
	return &MessageEdit{
		Channel: channelID,
		ID:      messageID,
	}
}

// SetContent is the same as setting the variable Content,
// except it doesn't take a pointer.
func (m *MessageEdit) SetContent(str string) *MessageEdit {
	m.Content = &str
	return m
}

// SetEmbeds is a convenience function for setting the embeds,
// so you can chain commands.
func (m *MessageEdit) SetEmbeds(embeds []*MessageEmbed) *MessageEdit {
	m.Embeds = embeds
	return m
}

// A MessageAttachment stores data for message attachments.
type MessageAttachment struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url"`
	Filename string `json:"filename"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Size     int    `json:"size"`
}

// MessageEmbedFooter is a part of a MessageEmbed struct.
type MessageEmbedFooter struct {
	Text         string `json:"text,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

// MessageEmbedImage is a part of a MessageEmbed struct.
type MessageEmbedImage struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// MessageEmbedThumbnail is a part of a MessageEmbed struct.
type MessageEmbedThumbnail struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// MessageEmbedVideo is a part of a MessageEmbed struct.
type MessageEmbedVideo struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// MessageEmbedProvider is a part of a MessageEmbed struct.
type MessageEmbedProvider struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

// MessageEmbedAuthor is a part of a MessageEmbed struct.
type MessageEmbedAuthor struct {
	URL          string `json:"url,omitempty"`
	Name         string `json:"name,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

// MessageEmbedField is a part of a MessageEmbed struct.
type MessageEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// An MessageEmbed stores data for message embeds.
type MessageEmbed struct {
	URL         string                 `json:"url,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Footer      *MessageEmbedFooter    `json:"footer,omitempty"`
	Image       *MessageEmbedImage     `json:"image,omitempty"`
	Thumbnail   *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Video       *MessageEmbedVideo     `json:"video,omitempty"`
	Provider    *MessageEmbedProvider  `json:"provider,omitempty"`
	Author      *MessageEmbedAuthor    `json:"author,omitempty"`
	Fields      []*MessageEmbedField   `json:"fields,omitempty"`

	//flag that tells the marshaller to marshal struct as nil
	marshalnil bool `json:"-"`
}

func (e *MessageEmbed) MarshalNil(flag bool) *MessageEmbed {
	e.marshalnil = flag
	return e

}

func (e *MessageEmbed) GetMarshalNil() bool {
	return e.marshalnil
}

func (e *MessageEmbed) MarshalJSON() ([]byte, error) {
	if e.marshalnil == true {
		return json.Marshal(nil)
	}
	type EmbedAlias MessageEmbed
	return json.Marshal(&struct{ *EmbedAlias }{EmbedAlias: (*EmbedAlias)(e)})
}

// MessageReactions holds a reactions object for a message.
type MessageReactions struct {
	Count int    `json:"count"`
	Me    bool   `json:"me"`
	Emoji *Emoji `json:"emoji"`
}

// ContentWithMentionsReplaced will replace all @<id> mentions with the
// username of the mention.
func (m *Message) ContentWithMentionsReplaced() (content string) {
	content = m.Content

	for _, user := range m.Mentions {
		content = strings.NewReplacer(
			"<@"+StrID(user.ID)+">", "@"+user.Username,
			"<@!"+StrID(user.ID)+">", "@"+user.Username,
		).Replace(content)
	}
	return
}

var patternChannels = regexp.MustCompile("<#[^>]*>")

// ContentWithMoreMentionsReplaced will replace all @<id> mentions with the
// username of the mention, but also role IDs and more.
func (m *Message) ContentWithMoreMentionsReplaced(s *Session) (content string, err error) {
	content = m.Content

	if !s.StateEnabled {
		content = m.ContentWithMentionsReplaced()
		return
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		content = m.ContentWithMentionsReplaced()
		return
	}

	for _, user := range m.Mentions {
		nick := user.Username

		member, err := s.State.Member(channel.GuildID, user.ID)
		if err == nil && member.Nick != "" {
			nick = member.Nick
		}

		content = strings.NewReplacer(
			"<@"+StrID(user.ID)+">", "@"+user.Username,
			"<@!"+StrID(user.ID)+">", "@"+nick,
		).Replace(content)
	}
	for _, roleID := range m.MentionRoles {
		role, err := s.State.Role(channel.GuildID, roleID)
		if err != nil || !role.Mentionable {
			continue
		}

		content = strings.Replace(content, "<@&"+StrID(role.ID)+">", "@"+role.Name, -1)
	}

	content = patternChannels.ReplaceAllStringFunc(content, func(mention string) string {
		id, err := strconv.ParseInt(mention[2:len(mention)-1], 10, 64)
		if err != nil {
			return mention
		}

		channel, err := s.State.Channel(id)
		if err != nil || channel.Type == ChannelTypeGuildVoice {
			return mention
		}

		return "#" + channel.Name
	})
	return
}

type AllowedMentionType string

const (
	AllowedMentionTypeRoles    AllowedMentionType = "roles"
	AllowedMentionTypeUsers    AllowedMentionType = "users"
	AllowedMentionTypeEveryone AllowedMentionType = "everyone"
)

type AllowedMentions struct {
	// Allowed mention types to parse from message content
	Parse []AllowedMentionType `json:"parse"`

	// Slice of role ids to mention
	Roles IDSlice `json:"roles"`

	// Slice of users to mention
	Users IDSlice `json:"users"`

	RepliedUser bool `json:"replied_user"`
}

// MessageReference contains reference data sent with crossposted messages
type MessageReference struct {
	MessageID int64 `json:"message_id,string"`
	ChannelID int64 `json:"channel_id,string"`
	GuildID   int64 `json:"guild_id,string,omitempty"`
}

// Reference returns MessageReference of given message
func (m *Message) Reference() *MessageReference {
	return &MessageReference{
		GuildID:   m.GuildID,
		ChannelID: m.ChannelID,
		MessageID: m.ID,
	}
}

// MessageInteraction contains information about the application command interaction which generated the message.
type MessageInteraction struct {
	ID   int64           `json:"id,string"`
	Type InteractionType `json:"type"`
	Name string          `json:"name"`
	User *User           `json:"user"`

	// Member is only present when the interaction is from a guild.
	Member *Member `json:"member"`
}
