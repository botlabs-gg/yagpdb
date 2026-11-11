package discordgo

import "testing"

func TestRoleMemberCount(t *testing.T) {
	defer func(old func(guildID, roleID int64) (int, error)) { RoleMemberCountFetcher = old }(RoleMemberCountFetcher)
	RoleMemberCountFetcher = func(guildID, roleID int64) (int, error) {
		return int(guildID + roleID), nil
	}

	if _, err := (&Role{ID: 2}).MemberCount(); err == nil {
		t.Fatal("expected an error for a role without a guild")
	}

	count, err := (&Role{ID: 2, GuildID: 1}).MemberCount()
	if err != nil {
		t.Fatal(err)
	}

	if count != 3 {
		t.Fatalf("expected the count from the fetcher, got %d", count)
	}
}
