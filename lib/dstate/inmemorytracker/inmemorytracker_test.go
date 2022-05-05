package inmemorytracker

import (
	"strconv"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var testSession = &discordgo.Session{ShardID: 0, ShardCount: 1}

const (
	initialTestGuildID   = 1
	initialTestChannelID = 10
	initialTestRoleID    = 100
	initialTestMemberID  = 1000

	intialTestThreadID = 10000
	testThreadID       = 10001
)

func createTestChannel(guildID int64, channelID int64, permissionsOverwrites []*discordgo.PermissionOverwrite) *discordgo.Channel {
	return &discordgo.Channel{
		ID:                   channelID,
		GuildID:              guildID,
		Name:                 "test channel-" + strconv.FormatInt(channelID, 10),
		Type:                 discordgo.ChannelTypeGuildText,
		PermissionOverwrites: permissionsOverwrites,
	}
}

func createTestUser(id int64) *discordgo.User {
	return &discordgo.User{
		ID:            id,
		Username:      "test member-" + strconv.FormatInt(id, 10),
		Discriminator: "0000",
	}
}

func createTestMember(guildID int64, id int64, roles []int64) *discordgo.Member {
	return &discordgo.Member{
		GuildID: guildID,
		Roles:   roles,
		User: &discordgo.User{
			ID:            id,
			Username:      "test member-" + strconv.FormatInt(id, 10),
			Discriminator: "0000",
		},
	}
}

func createTestState(conf TrackerConfig) *InMemoryTracker {
	state := NewInMemoryTracker(conf, 1)
	state.HandleEvent(testSession, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{
			ID:          initialTestGuildID,
			Name:        "test guild",
			OwnerID:     initialTestMemberID,
			MemberCount: 1,
			Members: []*discordgo.Member{
				createTestMember(0, initialTestMemberID, []int64{initialTestRoleID}),
			},
			Presences: []*discordgo.Presence{
				{User: createTestUser(initialTestMemberID)},
			},
			Channels: []*discordgo.Channel{
				createTestChannel(0, initialTestChannelID, nil),
			},
			Roles: []*discordgo.Role{
				{ID: initialTestRoleID},
			},
			Threads: []*discordgo.Channel{
				{
					ID:             intialTestThreadID,
					Name:           "test",
					Type:           discordgo.ChannelTypeGuildPublicThread,
					ParentID:       initialTestChannelID,
					ThreadMetadata: &discordgo.ThreadMetadata{},
				},
			},
		},
	})

	return state
}

func assertMemberExists(t *testing.T, tracker *InMemoryTracker, guildID int64, memberID int64, checkMember, checkPresence bool) {
	ms := tracker.GetMember(guildID, memberID)
	if ms == nil {
		t.Fatal("ms is nil")
	}

	if checkMember && ms.Member == nil {
		t.Fatal("ms.Member is nil")
	}

	if checkPresence && ms.Presence == nil {
		t.Fatal("ms.presence is nil")
	}
}

func TestGuildCreate(t *testing.T) {
	tracker := createTestState(TrackerConfig{})
	assertMemberExists(t, tracker, 1, initialTestMemberID, true, true)

	gs := tracker.GetGuild(initialTestGuildID)
	if gs == nil {
		t.Fatal("gs is nil")
	}

	if gs.GetRole(initialTestRoleID) == nil {
		t.Fatal("gc role is nil")
	}

	if gs.GetChannel(initialTestChannelID) == nil {
		t.Fatal("gc channel is nil")
	}

	if gs.GetThread(intialTestThreadID) == nil {
		t.Fatal("thread not found")
	}
}

func TestNoneExistantMember(t *testing.T) {
	tracker := createTestState(TrackerConfig{})
	ms := tracker.GetMember(1, 10001)
	if ms != nil {
		t.Fatal("ms is not nul, should be nil")
	}
}

func TestMemberAdd(t *testing.T) {
	tracker := createTestState(TrackerConfig{})

	tracker.HandleEvent(testSession, &discordgo.GuildMemberAdd{
		Member: createTestMember(1, 1001, nil),
	})

	assertMemberExists(t, tracker, 1, 1001, true, false)

	gs := tracker.GetGuild(1)
	if gs.MemberCount != 2 {
		t.Fatal("Member count not increased:", gs.MemberCount)
	}
}

func TestChannelUpdate(t *testing.T) {
	tracker := createTestState(TrackerConfig{})
	channel := tracker.GetGuild(initialTestGuildID).GetChannel(initialTestChannelID)
	if channel == nil {
		t.Fatal("channel not found")
	}

	updt := createTestChannel(1, initialTestChannelID, nil)
	updt.Name = "this is a new name!"

	tracker.HandleEvent(testSession, &discordgo.ChannelUpdate{
		Channel: updt,
	})

	channel = tracker.GetGuild(initialTestGuildID).GetChannel(initialTestChannelID)
	if channel == nil {
		t.Fatal("channel not found")
	}

	if channel.Name != updt.Name {
		t.Fatalf("channel was not updated: name: %s", channel.Name)
	}
}

func TestRoleUpdate(t *testing.T) {
	tracker := createTestState(TrackerConfig{})
	role := tracker.GetGuild(initialTestGuildID).GetRole(initialTestRoleID)
	if role == nil {
		t.Fatal("role not found")
	}

	updt := &discordgo.GuildRole{
		Role: &discordgo.Role{
			ID:   initialTestRoleID,
			Name: "new role name!",
		},
		GuildID: initialTestGuildID,
	}

	tracker.HandleEvent(testSession, &discordgo.GuildRoleUpdate{
		GuildRole: updt,
	})

	role = tracker.GetGuild(initialTestGuildID).GetRole(initialTestRoleID)
	if role == nil {
		t.Fatal("role not found")
	}

	if role.Name != updt.Role.Name {
		t.Fatalf("role was not updated: name: %s", role.Name)
	}
}

func TestThreadCreateDelete(t *testing.T) {
	tracker := createTestState(TrackerConfig{})

	updt := discordgo.Channel{
		GuildID:        initialTestGuildID,
		ParentID:       initialTestChannelID,
		ID:             testThreadID,
		ThreadMetadata: &discordgo.ThreadMetadata{},
		Name:           "test",
	}

	tracker.HandleEvent(testSession, &discordgo.ThreadCreate{
		Channel: updt,
	})

	thread := tracker.GetGuild(initialTestGuildID).GetThread(testThreadID)
	if thread == nil {
		t.Fatal("thread not found")
	}

	if thread.Name != updt.Name {
		t.Fatalf("thread was not updated: name: %s", thread.Name)
	}

	tracker.HandleEvent(testSession, &discordgo.ThreadDelete{
		ID:       updt.ID,
		GuildID:  updt.GuildID,
		ParentID: updt.ParentID,
		Type:     discordgo.ChannelTypeGuildPublicThread,
	})

	thread = tracker.GetGuild(initialTestGuildID).GetThread(testThreadID)
	if thread != nil {
		t.Fatal("thread should have been deleted")
	}
}

func TestThreadArchive(t *testing.T) {
	tracker := createTestState(TrackerConfig{})

	updt := discordgo.Channel{
		GuildID:        initialTestGuildID,
		ParentID:       initialTestChannelID,
		ID:             testThreadID,
		ThreadMetadata: &discordgo.ThreadMetadata{},
		Name:           "test",
	}

	tracker.HandleEvent(testSession, &discordgo.ThreadCreate{
		Channel: updt,
	})

	thread := tracker.GetGuild(initialTestGuildID).GetThread(testThreadID)
	if thread == nil {
		t.Fatal("thread not found")
	}

	if thread.Name != updt.Name {
		t.Fatalf("thread was not updated: name: %s", thread.Name)
	}

	cop := updt
	cop.ThreadMetadata.Archived = true

	tracker.HandleEvent(testSession, &discordgo.ThreadUpdate{
		Channel: cop,
	})

	thread = tracker.GetGuild(initialTestGuildID).GetThread(testThreadID)
	if thread != nil {
		t.Fatal("thread should have been removed")
	}
}

func TestThreadParentPerms(t *testing.T) {
	botID := initialTestMemberID + 1
	tracker := createTestState(TrackerConfig{
		BotMemberID: int64(botID),
	})

	tracker.HandleEvent(testSession, &discordgo.ChannelUpdate{
		Channel: createTestChannel(initialTestGuildID, initialTestChannelID, []*discordgo.PermissionOverwrite{
			{Type: discordgo.PermissionOverwriteTypeMember, ID: int64(botID), Allow: discordgo.PermissionViewChannel},
		}),
	})

	// add the bot member
	memberAdd := discordgo.Member{
		GuildID: initialTestGuildID,
		User: &discordgo.User{
			ID: int64(botID),
		},
	}
	tracker.HandleEvent(testSession, &discordgo.GuildMemberAdd{
		Member: &memberAdd,
	})

	thread := tracker.GetGuild(initialTestGuildID).GetThread(intialTestThreadID)
	if thread == nil {
		t.Fatal("thread not found")
	}

	// update the channel to remove the bot user
	chUpdt2 := createTestChannel(initialTestGuildID, initialTestChannelID, []*discordgo.PermissionOverwrite{
		{Type: discordgo.PermissionOverwriteTypeMember, ID: int64(botID), Deny: discordgo.PermissionViewChannel},
	})
	tracker.HandleEvent(testSession, &discordgo.ChannelUpdate{
		Channel: chUpdt2,
	})

	thread = tracker.GetGuild(initialTestGuildID).GetThread(testThreadID)
	if thread != nil {
		t.Fatal("thread should have been removed")
	}
}
