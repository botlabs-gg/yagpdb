package inmemorytracker

import (
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func createObfuscatedChannel(guildID, channelID int64) *discordgo.Channel {
	return &discordgo.Channel{
		ID:      channelID,
		GuildID: guildID,
		Name:    "___hidden___",
		Type:    discordgo.ChannelTypeGuildText,
		Flags:   discordgo.ChannelFlagsObfuscated,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{Type: discordgo.PermissionOverwriteTypeRole, ID: guildID, Deny: discordgo.PermissionViewChannel},
		},
	}
}

func TestGuildCreateSkipsObfuscatedChannels(t *testing.T) {
	const hiddenChannelID = 11

	tracker := NewInMemoryTracker(TrackerConfig{}, 1)
	tracker.HandleEvent(testSession, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{
			ID: initialTestGuildID,
			Channels: []*discordgo.Channel{
				createTestChannel(0, initialTestChannelID, nil),
				createObfuscatedChannel(0, hiddenChannelID),
			},
		},
	})

	gs := tracker.GetGuild(initialTestGuildID)
	if gs.GetChannel(initialTestChannelID) == nil {
		t.Fatal("visible channel should be in state")
	}

	if gs.GetChannel(hiddenChannelID) != nil {
		t.Fatal("obfuscated channel should not be in state")
	}
}

func TestObfuscatedChannelUpdateRemovesAndRestores(t *testing.T) {
	state := createTestState(TrackerConfig{ChannelMessageLen: 100})
	shard := state.getShard(0)

	state.HandleEvent(testSession, &discordgo.MessageCreate{Message: createTestMessage(50000, time.Now())})
	state.HandleEvent(testSession, &discordgo.MessageCreate{Message: createTestMessageIn(intialTestThreadID, 50001, time.Now())})

	state.HandleEvent(testSession, &discordgo.ChannelUpdate{Channel: createObfuscatedChannel(initialTestGuildID, initialTestChannelID)})

	gs := state.GetGuild(initialTestGuildID)
	if gs.GetChannel(initialTestChannelID) != nil {
		t.Fatal("obfuscated channel should have been removed")
	}

	if gs.GetThread(intialTestThreadID) != nil {
		t.Fatal("threads of the obfuscated channel should have been removed")
	}

	for _, id := range []int64{initialTestChannelID, intialTestThreadID} {
		if _, ok := shard.channelMessages[id]; ok {
			t.Fatalf("messages for channel %d should have been removed", id)
		}
	}

	state.HandleEvent(testSession, &discordgo.ChannelUpdate{Channel: createTestChannel(initialTestGuildID, initialTestChannelID, nil)})

	cs := state.GetGuild(initialTestGuildID).GetChannel(initialTestChannelID)
	if cs == nil {
		t.Fatal("channel should be back in state after a full update")
	}

	if cs.Name != "test channel-10" {
		t.Fatalf("channel was not restored with the full data: name: %s", cs.Name)
	}
}

func TestChannelUpdateBeforeAccessGrantKeepsChannel(t *testing.T) {
	const (
		hiddenChannelID = 40
		adminRoleID     = 41
		botID           = initialTestMemberID + 1
	)

	state := createTestState(TrackerConfig{BotMemberID: botID})
	state.HandleEvent(testSession, &discordgo.GuildMemberAdd{Member: createTestMember(initialTestGuildID, botID, nil)})
	state.HandleEvent(testSession, &discordgo.ChannelCreate{Channel: createObfuscatedChannel(initialTestGuildID, hiddenChannelID)})

	// discord sends the full channel before the member update that gave the bot an admin role
	state.HandleEvent(testSession, &discordgo.ChannelUpdate{Channel: createTestChannel(initialTestGuildID, hiddenChannelID, []*discordgo.PermissionOverwrite{
		{Type: discordgo.PermissionOverwriteTypeRole, ID: initialTestGuildID, Deny: discordgo.PermissionViewChannel},
	})})
	state.HandleEvent(testSession, &discordgo.GuildRoleCreate{GuildRole: &discordgo.GuildRole{
		GuildID: initialTestGuildID,
		Role:    &discordgo.Role{ID: adminRoleID, Permissions: discordgo.PermissionAdministrator},
	}})
	state.HandleEvent(testSession, &discordgo.GuildMemberUpdate{Member: createTestMember(initialTestGuildID, botID, []int64{adminRoleID})})

	if state.GetGuild(initialTestGuildID).GetChannel(hiddenChannelID) == nil {
		t.Fatal("channel the bot gained access to should be in state")
	}
}

func TestBotRoleUpdateRemovesChannelsWithoutAccess(t *testing.T) {
	const (
		categoryID       = 20
		visibleChildID   = 21
		hiddenCategoryID = 30
		hiddenChildID    = 31
		botID            = initialTestMemberID + 1
	)

	state := createTestState(TrackerConfig{BotMemberID: botID})

	state.HandleEvent(testSession, &discordgo.GuildRoleUpdate{GuildRole: &discordgo.GuildRole{
		GuildID: initialTestGuildID,
		Role:    &discordgo.Role{ID: initialTestRoleID, Permissions: discordgo.PermissionViewChannel},
	}})
	state.HandleEvent(testSession, &discordgo.GuildMemberAdd{Member: createTestMember(initialTestGuildID, botID, []int64{initialTestRoleID})})

	category := createTestChannel(initialTestGuildID, categoryID, nil)
	category.Type = discordgo.ChannelTypeGuildCategory
	visibleChild := createTestChannel(initialTestGuildID, visibleChildID, []*discordgo.PermissionOverwrite{
		{Type: discordgo.PermissionOverwriteTypeMember, ID: botID, Allow: discordgo.PermissionViewChannel},
	})
	visibleChild.ParentID = categoryID
	hiddenCategory := createTestChannel(initialTestGuildID, hiddenCategoryID, nil)
	hiddenCategory.Type = discordgo.ChannelTypeGuildCategory
	hiddenChild := createTestChannel(initialTestGuildID, hiddenChildID, nil)
	hiddenChild.ParentID = hiddenCategoryID

	for _, c := range []*discordgo.Channel{category, visibleChild, hiddenCategory, hiddenChild} {
		state.HandleEvent(testSession, &discordgo.ChannelCreate{Channel: c})
	}

	state.HandleEvent(testSession, &discordgo.GuildRoleUpdate{GuildRole: &discordgo.GuildRole{
		GuildID: initialTestGuildID,
		Role:    &discordgo.Role{ID: initialTestRoleID},
	}})

	gs := state.GetGuild(initialTestGuildID)
	for _, id := range []int64{initialTestChannelID, hiddenCategoryID, hiddenChildID} {
		if gs.GetChannel(id) != nil {
			t.Fatalf("channel %d should have been removed", id)
		}
	}

	if gs.GetThread(intialTestThreadID) != nil {
		t.Fatal("thread in a removed channel should have been removed")
	}

	for _, id := range []int64{categoryID, visibleChildID} {
		if gs.GetChannel(id) == nil {
			t.Fatalf("channel %d should still be in state", id)
		}
	}
}

func TestEveryoneRoleUpdateRemovesChannelsWithoutAccess(t *testing.T) {
	const botID = initialTestMemberID + 1

	state := createTestState(TrackerConfig{BotMemberID: botID})

	state.HandleEvent(testSession, &discordgo.GuildRoleCreate{GuildRole: &discordgo.GuildRole{
		GuildID: initialTestGuildID,
		Role:    &discordgo.Role{ID: initialTestGuildID, Permissions: discordgo.PermissionViewChannel},
	}})
	state.HandleEvent(testSession, &discordgo.GuildMemberAdd{Member: createTestMember(initialTestGuildID, botID, nil)})

	if state.GetGuild(initialTestGuildID).GetChannel(initialTestChannelID) == nil {
		t.Fatal("channel should be in state while everyone can view it")
	}

	state.HandleEvent(testSession, &discordgo.GuildRoleUpdate{GuildRole: &discordgo.GuildRole{
		GuildID: initialTestGuildID,
		Role:    &discordgo.Role{ID: initialTestGuildID},
	}})

	if state.GetGuild(initialTestGuildID).GetChannel(initialTestChannelID) != nil {
		t.Fatal("channel should have been removed once everyone lost view access")
	}
}
