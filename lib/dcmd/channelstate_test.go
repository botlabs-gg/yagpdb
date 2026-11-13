package dcmd

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

type guildOnlyState struct {
	dstate.StateTracker
	gs *dstate.GuildSet
}

func (s *guildOnlyState) GetGuild(guildID int64) *dstate.GuildSet {
	return s.gs
}

func TestAddChannelToState(t *testing.T) {
	const guildID, channelID = 1, 2

	state := &guildOnlyState{gs: &dstate.GuildSet{GuildState: dstate.GuildState{ID: guildID}}}
	called := 0
	sys := &System{State: state, AddChannelToState: func(gID, cID int64) {
		called++
		if gID != guildID || cID != channelID {
			t.Fatalf("unexpected ids: %d %d", gID, cID)
		}
		state.gs = &dstate.GuildSet{GuildState: dstate.GuildState{ID: guildID}, Channels: []dstate.ChannelState{{ID: channelID, GuildID: guildID}}}
	}}

	data, err := sys.FillDataInteraction(nil, &discordgo.Interaction{GuildID: guildID, ChannelID: channelID, Member: &discordgo.Member{User: &discordgo.User{ID: 3}}})
	if err != nil {
		t.Fatal(err)
	}

	if called != 1 || data.GuildData.CS == nil || data.GuildData.GS.GetChannel(channelID) == nil {
		t.Fatal("fetched channel should be used, along with the updated guild")
	}

	sys.AddChannelToState = func(int64, int64) {}
	if _, err := sys.FillDataInteraction(nil, &discordgo.Interaction{GuildID: guildID, ChannelID: 5}); err != ErrChannelNotFound {
		t.Fatalf("expected channel not found, got %v", err)
	}
}

func TestInteractionFromHiddenChannel(t *testing.T) {
	const guildID, channelID = 1, 2

	state := &guildOnlyState{gs: &dstate.GuildSet{GuildState: dstate.GuildState{ID: guildID}}}
	sys := &System{State: state, AddChannelToState: func(int64, int64) {
		t.Fatal("channels the bot cannot view should not be fetched")
	}}

	data, err := sys.FillDataInteraction(nil, &discordgo.Interaction{
		GuildID:        guildID,
		ChannelID:      channelID,
		Channel:        &discordgo.Channel{ID: channelID, Name: "hidden", Type: discordgo.ChannelTypeGuildText},
		AppPermissions: discordgo.PermissionSendMessages,
		Member:         &discordgo.Member{User: &discordgo.User{ID: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if data.GuildData.CS == nil || data.GuildData.CS.Name != "hidden" || data.GuildData.GS.GetChannel(channelID) == nil {
		t.Fatal("the channel from the interaction should be used")
	}

	if state.gs.GetChannel(channelID) != nil {
		t.Fatal("the channel from the interaction should not be added to state")
	}
}
