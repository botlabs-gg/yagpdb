package templates

import (
	"errors"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

// CtxChannel is almost a 1:1 copy of dstate.ChannelState, its needed because we cant axpose all those state methods
// we also cant use discordgo.Channel because that would likely break a lot of custom commands at this point.
type CtxChannel struct {
	// These fields never change
	ID        int64
	GuildID   int64
	IsPrivate bool
	IsThread  bool

	Name                 string                           `json:"name"`
	Type                 discordgo.ChannelType            `json:"type"`
	Topic                string                           `json:"topic"`
	NSFW                 bool                             `json:"nsfw"`
	Position             int                              `json:"position"`
	Bitrate              int                              `json:"bitrate"`
	PermissionOverwrites []*discordgo.PermissionOverwrite `json:"permission_overwrites"`
	ParentID             int64                            `json:"parent_id"`
	OwnerID              int64                            `json:"owner_id"`

	// The set of tags that can be used in a forum channel.
	AvailableTags []discordgo.ForumTag `json:"available_tags"`

	// The IDs of the set of tags that have been applied to a thread in a forum channel.
	AppliedTags []discordgo.ForumTag `json:"applied_tags"`
}

// CtxThreadStart is almost a 1:1 copy of discordgo.ThreadStart but with some added fields
type CtxThreadStart struct {
	Name                string                `json:"name"`
	AutoArchiveDuration int                   `json:"auto_archive_duration,omitempty"`
	Type                discordgo.ChannelType `json:"type,omitempty"`
	Invitable           bool                  `json:"invitable"`
	RateLimitPerUser    int                   `json:"rate_limit_per_user,omitempty"`

	Content *discordgo.MessageSend `json:"content,omitempty"`

	// NOTE: forum threads only - these are names not ids
	AppliedTagNames []string `json:"applied_tag_names,omitempty"`

	// NOTE: message threads only
	MessageID int64 `json:"message_id,omitempty"`
}

func (c *CtxChannel) Mention() (string, error) {
	if c == nil {
		return "", errors.New("channel not found")
	}
	return "<#" + discordgo.StrID(c.ID) + ">", nil
}

func CtxChannelFromCS(cs *dstate.ChannelState) *CtxChannel {

	cop := make([]*discordgo.PermissionOverwrite, len(cs.PermissionOverwrites))
	for i := 0; i < len(cs.PermissionOverwrites); i++ {
		cop[i] = &cs.PermissionOverwrites[i]
	}

	ctxChannel := &CtxChannel{
		ID:                   cs.ID,
		IsPrivate:            cs.IsPrivate(),
		IsThread:             cs.Type.IsThread(),
		GuildID:              cs.GuildID,
		Name:                 cs.Name,
		Type:                 cs.Type,
		Topic:                cs.Topic,
		NSFW:                 cs.NSFW,
		Position:             cs.Position,
		Bitrate:              cs.Bitrate,
		PermissionOverwrites: cop,
		ParentID:             cs.ParentID,
		OwnerID:              cs.OwnerID,
		AvailableTags:        cs.AvailableTags,
		AppliedTags:          cs.AppliedTags,
	}

	return ctxChannel
}
