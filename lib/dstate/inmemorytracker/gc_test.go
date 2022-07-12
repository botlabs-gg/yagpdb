package inmemorytracker

import (
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func createTestMessage(id int64, ts time.Time) *discordgo.Message {
	return &discordgo.Message{
		ID:        id,
		ChannelID: initialTestChannelID,
		GuildID:   initialTestGuildID,
		Content:   "test message",
		Timestamp: discordgo.Timestamp(ts.Format(time.RFC3339)),
	}
}

func TestGCMessages(t *testing.T) {
	state := createTestState(TrackerConfig{
		ChannelMessageLen:         2,
		ChannelMessageDur:         time.Hour,
		RemoveOfflineMembersAfter: time.Hour,
	})
	shard := state.getShard(0)

	state.HandleEvent(testSession, &discordgo.MessageCreate{
		Message: createTestMessage(10000, time.Date(2021, 5, 20, 10, 0, 0, 0, time.UTC)),
	})

	state.HandleEvent(testSession, &discordgo.MessageCreate{
		Message: createTestMessage(10001, time.Date(2021, 5, 20, 10, 0, 2, 0, time.UTC)),
	})

	// verify the contents now
	verifyMessages(t, state, initialTestChannelID, []int64{10000, 10001})

	// add another message that will be GC'd soon
	state.HandleEvent(testSession, &discordgo.MessageCreate{
		Message: createTestMessage(10002, time.Date(2021, 5, 20, 10, 0, 4, 0, time.UTC)),
	})
	verifyMessages(t, state, initialTestChannelID, []int64{10000, 10001, 10002})

	// run a gc, verifying max len works
	shard.gcTick(time.Date(2021, 5, 20, 10, 0, 2, 0, time.UTC), nil)
	verifyMessages(t, state, initialTestChannelID, []int64{10001, 10002})

	// run a gc verifying max age
	shard.gcTick(time.Date(2021, 5, 20, 11, 0, 3, 0, time.UTC), nil)
	verifyMessages(t, state, initialTestChannelID, []int64{10002})

	// run yet another one because why not
	shard.gcTick(time.Date(2021, 5, 20, 12, 0, 3, 0, time.UTC), nil)
	verifyMessages(t, state, initialTestChannelID, []int64{})
}

func verifyMessages(t *testing.T, state *InMemoryTracker, channelID int64, expectedResult []int64) {
	shard := state.getShard(0)

	messages, ok := shard.messages[channelID]
	if !ok {
		t.Fatal("emessages slice not present")
	}

	if messages.Len() != len(expectedResult) {
		t.Fatalf("mismatched lengths, got: %d, expected: %d", messages.Len(), len(expectedResult))
	}

	i := 0
	for e := messages.Front(); e != nil; e = e.Next() {

		cast := e.Value.(*dstate.MessageState)
		if cast.ID != expectedResult[i] {
			t.Fatalf("mismatched result at index [%d]: %d, expected %d", i, cast.ID, expectedResult[i])
		}

		i++
	}
}

func createGCTestMember(id int64, t time.Time, member *dstate.MemberFields, presence *dstate.PresenceFields) *WrappedMember {
	return &WrappedMember{
		lastUpdated: t,
		MemberState: dstate.MemberState{
			User:     *createTestUser(id),
			GuildID:  initialTestGuildID,
			Member:   member,
			Presence: presence,
		},
	}
}

func TestGCMembers(t *testing.T) {
	state := createTestState(TrackerConfig{
		ChannelMessageLen:         2,
		ChannelMessageDur:         time.Hour,
		RemoveOfflineMembersAfter: time.Hour,
	})
	shard := state.getShard(0)

	// createTestState adds a initial member, we overwrite it here for test reliability
	shard.members[initialTestGuildID] = map[int64]*WrappedMember{
		1000: createGCTestMember(1000, time.Date(2021, 5, 20, 10, 0, 0, 0, time.UTC), nil, nil),
		1001: createGCTestMember(1001, time.Date(2021, 5, 20, 10, 2, 0, 0, time.UTC), nil, &dstate.PresenceFields{Status: dstate.StatusIdle}),
		1002: createGCTestMember(1002, time.Date(2021, 5, 20, 10, 4, 0, 0, time.UTC), nil, &dstate.PresenceFields{Status: dstate.StatusOffline}),
	}

	// verify the contents now
	verifyMembers(t, state, initialTestGuildID, []int64{1000, 1001, 1002})

	// trigger a gc with no efffect
	shard.gcTick(time.Date(2021, 5, 20, 10, 0, 0, 0, time.UTC), nil)
	verifyMembers(t, state, initialTestGuildID, []int64{1000, 1001, 1002})

	// remove 1 member
	shard.gcTick(time.Date(2021, 5, 20, 11, 1, 0, 0, time.UTC), nil)
	verifyMembers(t, state, initialTestGuildID, []int64{1001, 1002})

	// remove the other one, making sure the online one stays
	shard.gcTick(time.Date(2021, 5, 20, 12, 1, 0, 0, time.UTC), nil)
	verifyMembers(t, state, initialTestGuildID, []int64{1001})
}

func verifyMembers(t *testing.T, state *InMemoryTracker, guildID int64, expectedResult []int64) {
	shard := state.getShard(0)

	members, ok := shard.members[guildID]
	if !ok {
		t.Fatal("members slice not present")
	}

	if len(members) != len(expectedResult) {
		t.Fatalf("mismatched lengths, got: %d, expected: %d", len(members), len(expectedResult))
	}

OUTER:
	for _, expecting := range expectedResult {
		for _, v := range members {
			if v.User.ID == expecting {
				continue OUTER
			}
		}

		// couldn't find this member
		t.Fatalf("couldn't find member: %d", expecting)
	}
}
