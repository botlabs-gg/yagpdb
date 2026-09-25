package inmemorytracker

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func benchGuildCreate(b *testing.B, n int) {
	members := make([]*discordgo.Member, n)
	presences := make([]*discordgo.Presence, n)
	for i := 0; i < n; i++ {
		id := int64(100000 + i)
		members[i] = createTestMember(5, id, nil)
		presences[i] = &discordgo.Presence{User: createTestUser(id)}
	}

	gc := &discordgo.GuildCreate{Guild: &discordgo.Guild{
		ID: 5, Name: "big", MemberCount: n, Members: members, Presences: presences,
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker := NewInMemoryTracker(TrackerConfig{}, 1)
		tracker.HandleEvent(testSession, gc)
	}
}

func BenchmarkGuildCreate1000(b *testing.B)  { benchGuildCreate(b, 1000) }
func BenchmarkGuildCreate5000(b *testing.B)  { benchGuildCreate(b, 5000) }
func BenchmarkGuildCreate10000(b *testing.B) { benchGuildCreate(b, 10000) }
