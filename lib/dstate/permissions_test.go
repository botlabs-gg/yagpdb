package dstate

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestGuildPermissions(t *testing.T) {
	gs := &GuildState{
		ID:      1,
		OwnerID: 0,
	}

	roles := []discordgo.Role{
		{
			Name:        "Admin",
			ID:          10,
			Permissions: discordgo.PermissionAdministrator,
		},
		{
			Name:        "Mod",
			ID:          11,
			Permissions: discordgo.PermissionBanMembers | discordgo.PermissionManageNicknames | discordgo.PermissionManageMessages,
		},
		{
			Name:        "everyone",
			ID:          1,
			Permissions: discordgo.PermissionSendMessages,
		},
	}

	perms := CalculatePermissions(gs, roles, nil, 100, []int64{10})
	expectPerms(t, perms, AllPermissions)

	perms = CalculatePermissions(gs, roles, nil, 100, []int64{})
	expectPerms(t, perms, discordgo.PermissionSendMessages)

	perms = CalculatePermissions(gs, roles, nil, 100, []int64{1111})
	expectPerms(t, perms, discordgo.PermissionSendMessages)

	perms = CalculatePermissions(gs, roles, nil, 100, []int64{11})
	expectPerms(t, perms, discordgo.PermissionBanMembers|discordgo.PermissionManageNicknames|discordgo.PermissionManageMessages|discordgo.PermissionSendMessages)
}

func TestChannelPermissions(t *testing.T) {
	gs := &GuildState{
		ID:      1,
		OwnerID: 0,
	}

	roles := []discordgo.Role{
		{
			Name:        "Admin",
			ID:          10,
			Permissions: discordgo.PermissionAdministrator,
		},
		{
			Name:        "Mod",
			ID:          11,
			Permissions: discordgo.PermissionBanMembers | discordgo.PermissionManageNicknames | discordgo.PermissionManageMessages,
		},
		{
			Name:        "Muted",
			ID:          12,
			Permissions: 0,
		},
		{
			Name:        "everyone",
			ID:          1,
			Permissions: discordgo.PermissionSendMessages,
		},
	}

	overwrites := []discordgo.PermissionOverwrite{
		{
			Type:  discordgo.PermissionOverwriteTypeRole,
			ID:    12,
			Deny:  discordgo.PermissionSendMessages,
			Allow: discordgo.PermissionAdministrator,
		},
		{
			Type:  discordgo.PermissionOverwriteTypeMember,
			ID:    100,
			Allow: discordgo.PermissionEmbedLinks,
			Deny:  discordgo.PermissionAddReactions,
		},
		{
			Type:  discordgo.PermissionOverwriteTypeRole,
			ID:    1,
			Allow: discordgo.PermissionSendMessages | discordgo.PermissionAddReactions,
		},
	}

	perms := CalculatePermissions(gs, roles, overwrites, 101, []int64{12})
	expectPerms(t, perms, discordgo.PermissionAddReactions)

	perms = CalculatePermissions(gs, roles, overwrites, 100, []int64{12})
	expectPerms(t, perms, discordgo.PermissionEmbedLinks)

	perms = CalculatePermissions(gs, roles, overwrites, 100, []int64{})
	expectPerms(t, perms, discordgo.PermissionEmbedLinks|discordgo.PermissionSendMessages)

}

func expectPerms(t *testing.T, actual int64, expected int64) {
	if actual != expected {
		t.Fatalf("incorrect perms, got: %d, expected: %d", actual, expected)
	}
}
