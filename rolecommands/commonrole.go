package rolecommands

import (
	"context"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	schEvtsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
	"github.com/tidwall/buntdb"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// CommonRoleSettings helps the bot logic by abstracting away the type of the role settings
// Right now it's only abstracting away whether its a menu connected to a rolegroup or a standalone menu
type CommonRoleSettings struct {
	// Either the menu or group is provided, we use the settings from one of them
	ParentMenu  *models.RoleMenu
	ParentGroup *models.RoleGroup

	// these are provided depending on wether the parent is a menu in standalone mode or not
	RoleCmd    *models.RoleCommand
	MenuOption *models.RoleMenuOption

	// convience fields taken from above for ease of use
	RoleId int64

	ParentWhitelistRoles []int64
	ParentBlacklistRoles []int64
	ParentGroupMode      int

	// White list and blacklist roles for this specific role
	WhitelistRoles []int64
	BlacklistRoles []int64
}

// Assumes relationships are loaded for non standalone menus
// Falls back to CommonRoleFromRoleCommand if there is a rolegroup attached to the menu
func CommonRoleFromRoleMenuCommand(rm *models.RoleMenu, option *models.RoleMenuOption) *CommonRoleSettings {
	if rm.RoleGroupID.Valid {
		// this is not a standalone menu, its tied to a rolegroup
		return CommonRoleFromRoleCommand(rm.R.RoleGroup, option.R.RoleCommand)
	}

	return &CommonRoleSettings{
		ParentMenu:  rm,
		ParentGroup: nil,

		RoleCmd:    nil,
		MenuOption: option,

		RoleId:         option.StandaloneRoleID.Int64,
		WhitelistRoles: option.WhitelistRoles,
		BlacklistRoles: option.BlacklistRoles,

		ParentWhitelistRoles: rm.StandaloneWhitelistRoles,
		ParentBlacklistRoles: rm.StandaloneBlacklistRoles,
		ParentGroupMode:      int(rm.StandaloneMode.Int16),
	}
}

// Assumes relationships are loaded, group is optional
func CommonRoleFromRoleCommand(group *models.RoleGroup, cmd *models.RoleCommand) *CommonRoleSettings {

	settings := &CommonRoleSettings{
		ParentMenu:  nil,
		ParentGroup: group,

		RoleCmd:    cmd,
		MenuOption: nil,

		RoleId:         cmd.Role,
		WhitelistRoles: cmd.RequireRoles,
		BlacklistRoles: cmd.IgnoreRoles,
	}

	if group != nil {
		settings.ParentGroupMode = int(group.Mode)
		settings.ParentWhitelistRoles = group.RequireRoles
		settings.ParentBlacklistRoles = group.IgnoreRoles
	}

	return settings
}

type ModeSettings struct {
	Mode                  int64
	MultipleMax           int64
	MultipleMin           int64
	SingleAutoToggleOff   bool
	SingleRequireOne      bool
	TemporaryRoleDuration int
}

func (c *CommonRoleSettings) ModeSettings() *ModeSettings {
	if c.ParentGroup != nil {
		return &ModeSettings{
			Mode:                  c.ParentGroup.Mode,
			MultipleMin:           c.ParentGroup.MultipleMin,
			MultipleMax:           c.ParentGroup.MultipleMax,
			SingleAutoToggleOff:   c.ParentGroup.SingleAutoToggleOff,
			SingleRequireOne:      c.ParentGroup.SingleRequireOne,
			TemporaryRoleDuration: c.ParentGroup.TemporaryRoleDuration,
		}
	} else {
		return &ModeSettings{
			Mode:                  int64(c.ParentMenu.StandaloneMode.Int16),
			MultipleMin:           int64(c.ParentMenu.StandaloneMultipleMin.Int),
			MultipleMax:           int64(c.ParentMenu.StandaloneMultipleMax.Int),
			SingleAutoToggleOff:   c.ParentMenu.StandaloneSingleAutoToggleOff.Bool,
			SingleRequireOne:      c.ParentMenu.StandaloneSingleRequireOne.Bool,
			TemporaryRoleDuration: 0,
		}
	}
}

func (c *CommonRoleSettings) AllGroupRoles(ctx context.Context) []*CommonRoleSettings {
	result := make([]*CommonRoleSettings, 0, 20)
	if c.ParentMenu != nil {
		for _, v := range c.ParentMenu.R.RoleMenuOptions {
			result = append(result, CommonRoleFromRoleMenuCommand(c.ParentMenu, v))
		}
	} else {
		for _, cmd := range c.ParentGroup.R.RoleCommands {
			result = append(result, CommonRoleFromRoleCommand(c.ParentGroup, cmd))
		}
	}
	return result
}

func (c *CommonRoleSettings) CanRole(ctx context.Context, ms *dstate.MemberState) (can bool, err error) {
	onCD := false

	// First check cooldown
	if c.ParentGroupMode == GroupModeSingle {
		err = cooldownsDB.Update(func(tx *buntdb.Tx) error {
			_, replaced, _ := tx.Set(discordgo.StrID(ms.User.ID), "1", &buntdb.SetOptions{Expires: true, TTL: time.Second * 1})
			if replaced {
				onCD = true
			}
			return nil
		})

		if onCD {
			return false, NewSimpleError("You're changing roles too quickly. Please wait a second and try again.")
		}
	}

	if len(c.WhitelistRoles) > 0 {
		if !CheckRequiredRoles(c.WhitelistRoles, ms.Member.Roles) {
			return false, NewSimpleError("This self assignable role has been configured to require another role by the server admins.")
		}
	}

	if len(c.BlacklistRoles) > 0 {
		if err := CheckIgnoredRoles(c.BlacklistRoles, ms.Member.Roles); err != nil {
			return false, err
		}
	}

	if c.ParentMenu != nil || c.ParentGroup != nil {
		return c.ParentCanRole(ctx, ms)
	}

	// This command belongs to a group, let the group handle it
	// if cmd.R.RoleGroup != nil {
	// 	return GroupCanRole(ctx, ms, cmd)
	// }

	return true, nil
}

func (c *CommonRoleSettings) ParentCanRole(ctx context.Context, ms *dstate.MemberState) (can bool, err error) {
	if len(c.ParentWhitelistRoles) > 0 {
		if !CheckRequiredRoles(c.ParentWhitelistRoles, ms.Member.Roles) {
			err = NewSimpleError("You don't have a required role for this self-assignable role group.")
			return false, err
		}
	}

	if len(c.ParentBlacklistRoles) > 0 {
		if err = CheckIgnoredRoles(c.ParentBlacklistRoles, ms.Member.Roles); err != nil {
			return false, err
		}
	}

	// Default behaviour of groups is no more restrictions than whitelist and ignore roles
	if c.ParentGroupMode == GroupModeNone {
		return true, nil
	}

	// First retrieve role commands for this group
	commands := c.AllGroupRoles(ctx)
	modeSettings := c.ModeSettings()

	if c.ParentGroupMode == GroupModeSingle {
		// If user already has role it's attempting to give itself, assume were trying to remove it
		if common.ContainsInt64Slice(ms.Member.Roles, c.RoleId) {
			if modeSettings.SingleRequireOne {
				return false, NewSimpleError("Need at least one role in this group/rolemenu")
			}

			return true, nil
		}

		// Check if the user has any other role commands in this group
		for _, v := range commands {
			if common.ContainsInt64Slice(ms.Member.Roles, v.RoleId) {
				if !modeSettings.SingleAutoToggleOff {
					return false, NewSimpleError("Max 1 role in this group/rolemenu is allowed")
				}
			}
		}

		// If we got here then we can
		return true, err
	}

	// Handle multiple mode

	// Count roles in group and check against min-max
	// also check if we already have said role
	hasRoles := 0
	hasTargetRole := false
	for _, role := range commands {
		if common.ContainsInt64Slice(ms.Member.Roles, role.RoleId) {
			hasRoles++
			if role.RoleId == c.RoleId {
				hasTargetRole = true
			}
		}
	}

	if hasTargetRole {
		if hasRoles-1 < int(modeSettings.MultipleMin) {
			err = NewLmitError("Minimum of `%d` roles required in this group", int(modeSettings.MultipleMin))
			return false, err
		}
	} else {
		if hasRoles+1 > int(modeSettings.MultipleMax) {
			err = NewLmitError("Maximum of `%d` roles allowed in this group", int(modeSettings.MultipleMax))
			return false, err
		}
	}

	// If we got here then all checks passed
	return true, nil
}

// AssignRole attempts to assign the given role command, returns an error if the role does not exists
// or is unable to receie said role
// It also calls c.CanRole to check if we can assign it beforehand
func (c *CommonRoleSettings) CheckToggleRole(ctx context.Context, ms *dstate.MemberState) (gaveRole bool, err error) {
	if can, err := c.CanRole(ctx, ms); !can {
		return false, err
	}

	// This command belongs to a group/menu, let the group handle it
	if c.ParentGroup != nil || c.ParentMenu != nil {
		return c.GroupToggleRole(ctx, ms)
	}

	// This is a single command, just toggle it
	return c.ToggleRole(ms)
}

// ToggleRole toggles the role of a guildmember, adding it if the member does not have the role and removing it if they do
func (c *CommonRoleSettings) ToggleRole(ms *dstate.MemberState) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(ms.Member.Roles, c.RoleId) {
		err = common.BotSession.GuildMemberRoleRemove(ms.GuildID, ms.User.ID, c.RoleId)
		return false, err
	}

	err = common.BotSession.GuildMemberRoleAdd(ms.GuildID, ms.User.ID, c.RoleId)
	return true, err
}

func (c *CommonRoleSettings) GroupToggleRole(ctx context.Context, ms *dstate.MemberState) (gaveRole bool, err error) {
	// Default behaviour of groups is no more restrictions than reuiqred and ignore roles
	if c.ParentGroupMode != GroupModeSingle {
		// We already passed all checks
		gaveRole, err = c.ToggleRole(ms)
		if gaveRole && err == nil {
			err = c.MaybeScheduleRoleRemoval(ctx, ms)
		}
		return gaveRole, err
	}

	// If user already has role it's attempting to give itself
	if common.ContainsInt64Slice(ms.Member.Roles, c.RoleId) {
		err = common.BotSession.GuildMemberRoleRemove(ms.GuildID, ms.User.ID, c.RoleId)
		return false, err
	}

	// Check if the user has any other role commands in this group
	commands := c.AllGroupRoles(ctx)
	for _, v := range commands {
		if common.ContainsInt64Slice(ms.Member.Roles, v.RoleId) {
			if c.ModeSettings().SingleAutoToggleOff {
				common.BotSession.GuildMemberRoleRemove(ms.GuildID, ms.User.ID, v.RoleId)
			} else {
				return false, NewCommonRoleError("Max 1 role in **%s** is allowed", c)
			}
		}
	}

	// Finally give the role
	err = common.BotSession.GuildMemberRoleAdd(ms.GuildID, ms.User.ID, c.RoleId)
	if err == nil {
		err = c.MaybeScheduleRoleRemoval(ctx, ms)
	}
	return true, err
}

func (c *CommonRoleSettings) AssignRole(ctx context.Context, ms *dstate.MemberState) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(ms.Member.Roles, c.RoleId) {
		return false, nil
	}

	return c.CheckToggleRole(ctx, ms)
}

func (c *CommonRoleSettings) RemoveRole(ctx context.Context, ms *dstate.MemberState) (removedRole bool, err error) {
	if !common.ContainsInt64Slice(ms.Member.Roles, c.RoleId) {
		return false, nil
	}

	given, err := c.CheckToggleRole(ctx, ms)
	return !given, err
}

func (c *CommonRoleSettings) MaybeScheduleRoleRemoval(ctx context.Context, ms *dstate.MemberState) error {
	modeSettings := c.ModeSettings()
	temporaryDuration := modeSettings.TemporaryRoleDuration
	if temporaryDuration == 0 || c.ParentGroup == nil {
		return nil
	}

	// remove existing role removal events for this role
	_, err := schEvtsModels.ScheduledEvents(qm.Where("event_name='remove_member_role' AND  guild_id = ? AND (data->>'user_id')::bigint = ? AND (data->>'role_id')::bigint = ?", ms.GuildID, ms.User.ID, c.RoleId)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return err
	}

	// add the scheduled event for it
	err = scheduledevents2.ScheduleEvent("remove_member_role", ms.GuildID, time.Now().Add(time.Duration(temporaryDuration)*time.Minute), &ScheduledMemberRoleRemoveData{
		GuildID: ms.GuildID,
		GroupID: c.ParentGroup.ID,
		UserID:  ms.User.ID,
		RoleID:  c.RoleId,
	})

	if err != nil {
		return err
	}

	return nil
}
