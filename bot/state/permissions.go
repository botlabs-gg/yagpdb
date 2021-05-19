package state

import "github.com/jonas747/discordgo"

const AllPermissions int64 = ^0

// CalculatePermissions calculates a members permissions
func CalculatePermissions(g *CachedGuild, guildRoles []*CachedRole, overwrites []discordgo.PermissionOverwrite, memberID int64, roles []int64) (perms int64) {
	if g.OwnerID == memberID {
		return AllPermissions
	}

	// Check guild scope permissions

	// everyone role first
	for _, role := range guildRoles {
		if role.ID == g.ID {
			perms |= int64(role.Permissions)
			break
		}
	}

	// member roles
	for _, role := range guildRoles {
		for _, roleID := range roles {
			if role.ID == roleID {
				perms |= int64(role.Permissions)
				break
			}
		}
	}

	// Administrator bypasses channel overrides
	if perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return AllPermissions
	}

	if len(overwrites) == 0 {
		return perms
	}

	// Apply chanel overwrites

	// Apply @everyone overrides from the channel.
	for _, overwrite := range overwrites {
		if g.ID == overwrite.ID {
			perms &= ^int64(overwrite.Deny)
			perms |= int64(overwrite.Allow)
			break
		}
	}

	denies := int64(0)
	allows := int64(0)

	// Member overwrites can override role overrides, so do two passes with roles first
	for _, overwrite := range overwrites {
		for _, roleID := range roles {
			if overwrite.Type == "role" && roleID == overwrite.ID {
				denies |= int64(overwrite.Deny)
				allows |= int64(overwrite.Allow)
				break
			}
		}
	}

	perms &= ^int64(denies)
	perms |= int64(allows)

	for _, overwrite := range overwrites {
		if overwrite.Type == "member" && overwrite.ID == memberID {
			perms &= ^int64(overwrite.Deny)
			perms |= int64(overwrite.Allow)
			break
		}
	}

	return perms
}
