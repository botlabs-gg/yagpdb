package templates

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
)

// CtxChannel is almost a 1:1 copy of dstate.ChannelState, its needed because we cant axpose all those state methods
// we also cant use discordgo.Channel because that would likely break a lot of custom commands at this point.
type CtxChannel struct {
	// These fields never change
	ID        int64
	GuildID   int64
	IsPrivate bool

	Name                 string                           `json:"name"`
	Type                 discordgo.ChannelType            `json:"type"`
	Topic                string                           `json:"topic"`
	LastMessageID        int64                            `json:"last_message_id"`
	NSFW                 bool                             `json:"nsfw"`
	Position             int                              `json:"position"`
	Bitrate              int                              `json:"bitrate"`
	PermissionOverwrites []*discordgo.PermissionOverwrite `json:"permission_overwrites"`
	ParentID             int64                            `json:"parent_id"`
}

func CtxChannelFromCS(cs *dstate.ChannelState) *CtxChannel {
	ctxChannel := &CtxChannel{
		ID:                   cs.ID,
		IsPrivate:            cs.IsPrivate,
		Name:                 cs.Name,
		Type:                 cs.Type,
		Topic:                cs.Topic,
		LastMessageID:        cs.LastMessageID,
		NSFW:                 cs.NSFW,
		Position:             cs.Position,
		Bitrate:              cs.Bitrate,
		PermissionOverwrites: cs.PermissionOverwrites,
		ParentID:             cs.ParentID,
	}

	if !cs.IsPrivate {
		ctxChannel.GuildID = cs.Guild.ID
	}

	return ctxChannel
}

func CtxChannelFromCSLocked(cs *dstate.ChannelState) *CtxChannel {
	cs.Owner.RLock()
	defer cs.Owner.RUnlock()

	ctxChannel := &CtxChannel{
		ID:                   cs.ID,
		IsPrivate:            cs.IsPrivate,
		Name:                 cs.Name,
		Type:                 cs.Type,
		Topic:                cs.Topic,
		LastMessageID:        cs.LastMessageID,
		NSFW:                 cs.NSFW,
		Position:             cs.Position,
		Bitrate:              cs.Bitrate,
		PermissionOverwrites: cs.PermissionOverwrites,
		ParentID:             cs.ParentID,
	}

	if !cs.IsPrivate {
		ctxChannel.GuildID = cs.Guild.ID
	}

	return ctxChannel
}
