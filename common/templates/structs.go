package templates

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v3"
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
	NSFW                 bool                             `json:"nsfw"`
	Position             int                              `json:"position"`
	Bitrate              int                              `json:"bitrate"`
	PermissionOverwrites []*discordgo.PermissionOverwrite `json:"permission_overwrites"`
	ParentID             int64                            `json:"parent_id"`
}

func (c *CtxChannel) Mention() string {
	return "<#" + discordgo.StrID(c.ID) + ">"
}

func CtxChannelFromCS(cs *dstate.ChannelState) *CtxChannel {

	cop := make([]*discordgo.PermissionOverwrite, len(cs.PermissionOverwrites))
	for i := 0; i < len(cs.PermissionOverwrites); i++ {
		cop[i] = &cs.PermissionOverwrites[i]
	}

	ctxChannel := &CtxChannel{
		ID:                   cs.ID,
		IsPrivate:            cs.IsPrivate(),
		GuildID:              cs.GuildID,
		Name:                 cs.Name,
		Type:                 cs.Type,
		Topic:                cs.Topic,
		NSFW:                 cs.NSFW,
		Position:             cs.Position,
		Bitrate:              cs.Bitrate,
		PermissionOverwrites: cop,
		ParentID:             cs.ParentID,
	}

	return ctxChannel
}
