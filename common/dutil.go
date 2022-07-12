package common

import (
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

// IsRoleAbove returns wether role a is above b, checking positions first, and if they're the same
// (both being 1, new roles always have 1 as position)
// then it checjs by lower id
func IsRoleAbove(a, b *discordgo.Role) bool {
	if a.Position != b.Position {
		return a.Position > b.Position
	}

	if a.ID == b.ID {
		return false
	}

	return a.ID < b.ID
}

// Channels are a collection of Channels
type DiscordChannels []*discordgo.Channel

func (r DiscordChannels) Len() int {
	return len(r)
}

func (r DiscordChannels) Less(i, j int) bool {
	return r[i].Position < r[j].Position
}

func (r DiscordChannels) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type DiscordRoles []*discordgo.Role

func (r DiscordRoles) Len() int {
	return len(r)
}

func (r DiscordRoles) Less(i, j int) bool {
	return IsRoleAbove(r[i], r[j])
}

func (r DiscordRoles) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// ChannelOrThreadParentID returns either cs.ID for channels or cs.ParentID for threads
func ChannelOrThreadParentID(cs *dstate.ChannelState) int64 {
	if cs.Type.IsThread() {
		return cs.ParentID
	}

	return cs.ID

}
