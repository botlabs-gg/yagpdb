package dstate

import "github.com/botlabs-gg/yagpdb/v2/lib/discordgo"

const AllPermissions int64 = discordgo.PermissionAll

// Apply this mask to channel permissions to filter them out
// discord performs no server side validation so this is needed
// as to not run into some really weird situations
const ChannelPermsMask = ^(discordgo.PermissionAdministrator |
	discordgo.PermissionManageServer |
	discordgo.PermissionChangeNickname |
	discordgo.PermissionManageServer |
	discordgo.PermissionManageRoles |
	discordgo.PermissionKickMembers |
	discordgo.PermissionBanMembers)

// CalculatePermissions calculates the full permissions of a user
func CalculatePermissions(g *GuildState, guildRoles []discordgo.Role, overwrites []discordgo.PermissionOverwrite, memberID int64, roles []int64) (perms int64) {
	perms = CalculateBasePermissions(g.ID, g.OwnerID, guildRoles, memberID, roles)
	perms = ApplyChannelPermissions(perms, g.ID, overwrites, memberID, roles)
	return perms
}

// CalculateBasePermissions calculates the guild scope permissions, excluding channel overwrites
func CalculateBasePermissions(guildID int64, ownerID int64, guildRoles []discordgo.Role, memberID int64, roles []int64) (perms int64) {
	if ownerID == memberID {
		return AllPermissions
	}

	// everyone role first
	for _, role := range guildRoles {
		if role.ID == guildID {
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

	return perms
}

// ApplyChannelPermissions applies the channel permission overwrites to the input perms
func ApplyChannelPermissions(perms int64, guildID int64, overwrites []discordgo.PermissionOverwrite, memberID int64, roles []int64) int64 {
	if len(overwrites) == 0 {
		return perms
	}

	// If user is admin or owner, overrides dont apply
	if perms == AllPermissions {
		return perms
	}

	// Apply chanel overwrites

	// Apply @everyone overrides from the channel.
	for _, overwrite := range overwrites {
		if guildID == overwrite.ID {
			perms &= ^int64(overwrite.Deny & ChannelPermsMask)
			perms |= int64(overwrite.Allow & ChannelPermsMask)
			break
		}
	}

	denies := int64(0)
	allows := int64(0)

	// Member overwrites can override role overrides, so do two passes with roles first
	for _, overwrite := range overwrites {
		for _, roleID := range roles {
			if overwrite.Type == discordgo.PermissionOverwriteTypeRole && roleID == overwrite.ID {
				denies |= int64(overwrite.Deny & ChannelPermsMask)
				allows |= int64(overwrite.Allow & ChannelPermsMask)
				break
			}
		}
	}

	perms &= ^int64(denies)
	perms |= int64(allows)

	for _, overwrite := range overwrites {
		if overwrite.Type == discordgo.PermissionOverwriteTypeMember && overwrite.ID == memberID {
			perms &= ^int64(overwrite.Deny & ChannelPermsMask)
			perms |= int64(overwrite.Allow & ChannelPermsMask)
			break
		}
	}

	return perms
}
