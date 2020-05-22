// rolecommands is a plugin which allows users to assign roles to themselves
package rolecommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEvtsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/tidwall/buntdb"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "RoleCommands",
		SysName:  "role_commands",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

const (
	GroupModeNone = iota
	GroupModeSingle
	GroupModeMultiple
)

const (
	RoleMenuStateSettingUp              = 0
	RoleMenuStateDone                   = 1
	RoleMenuStateEditingOptionSelecting = 2
	RoleMenuStateEditingOptionReplacing = 3
)

var (
	_ common.Plugin            = (*Plugin)(nil)
	_ web.Plugin               = (*Plugin)(nil)
	_ bot.BotInitHandler       = (*Plugin)(nil)
	_ commands.CommandProvider = (*Plugin)(nil)

	cooldownsDB *buntdb.DB
)

func RegisterPlugin() {
	cooldownsDB, _ = buntdb.Open(":memory:")

	p := &Plugin{}
	common.RegisterPlugin(p)

	common.InitSchemas("rolecommands", DBSchemas...)
}

func FindToggleRole(ctx context.Context, ms *dstate.MemberState, name string) (gaveRole bool, err error) {
	cmd, err := models.RoleCommands(qm.Where("guild_id=?", ms.Guild.ID), qm.Where("name ILIKE ?", name), qm.Load("RoleGroup.RoleCommands")).OneG(ctx)
	if err != nil {
		return false, err
	}

	return CheckToggleRole(ctx, ms, cmd)
}

func CanRole(ctx context.Context, ms *dstate.MemberState, cmd *models.RoleCommand) (can bool, err error) {
	onCD := false

	// First check cooldown
	if cmd.R.RoleGroup != nil && cmd.R.RoleGroup.Mode == GroupModeSingle {
		err = cooldownsDB.Update(func(tx *buntdb.Tx) error {
			_, replaced, _ := tx.Set(discordgo.StrID(ms.ID), "1", &buntdb.SetOptions{Expires: true, TTL: time.Second * 1})
			if replaced {
				onCD = true
			}
			return nil
		})

		if onCD {
			return false, NewSimpleError("You're on cooldown")
		}
	}

	if err := CanAssignRoleCmdTo(cmd, ms.Roles); err != nil {
		return false, err
	}

	// This command belongs to a group, let the group handle it
	if cmd.R.RoleGroup != nil {
		return GroupCanRole(ctx, ms, cmd)
	}

	return true, nil
}

// AssignRole attempts to assign the given role command, returns an error if the role does not exists
// or is unable to receie said role
func CheckToggleRole(ctx context.Context, ms *dstate.MemberState, cmd *models.RoleCommand) (gaveRole bool, err error) {
	if can, err := CanRole(ctx, ms, cmd); !can {
		return false, err
	}

	// This command belongs to a group, let the group handle it
	if cmd.R.RoleGroup != nil {
		return GroupToggleRole(ctx, ms, cmd)
	}

	// This is a single command, just toggle it
	return ToggleRole(ms, cmd.Role)
}

// ToggleRole toggles the role of a guildmember, adding it if the member does not have the role and removing it if they do
func ToggleRole(ms *dstate.MemberState, role int64) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(ms.Roles, role) {
		err = common.BotSession.GuildMemberRoleRemove(ms.Guild.ID, ms.ID, role)
		return false, err
	}

	err = common.BotSession.GuildMemberRoleAdd(ms.Guild.ID, ms.ID, role)
	return true, err
}

func AssignRole(ctx context.Context, ms *dstate.MemberState, cmd *models.RoleCommand) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(ms.Roles, cmd.Role) {
		return false, nil
	}

	return CheckToggleRole(ctx, ms, cmd)
}

func RemoveRole(ctx context.Context, ms *dstate.MemberState, cmd *models.RoleCommand) (removedRole bool, err error) {
	if !common.ContainsInt64Slice(ms.Roles, cmd.Role) {
		return false, nil
	}

	given, err := CheckToggleRole(ctx, ms, cmd)
	return !given, err
}

func GroupCanRole(ctx context.Context, ms *dstate.MemberState, targetRole *models.RoleCommand) (can bool, err error) {
	rg := targetRole.R.RoleGroup

	if len(rg.RequireRoles) > 0 {
		if !CheckRequiredRoles(rg.RequireRoles, ms.Roles) {
			err = NewSimpleError("You don't have a required role for this self-assignable role group.")
			return false, err
		}
	}

	if len(rg.IgnoreRoles) > 0 {
		if err = CheckIgnoredRoles(rg.IgnoreRoles, ms.Roles); err != nil {
			return false, err
		}
	}

	// Default behaviour of groups is no more restrictions than reuiqred and ignore roles
	if rg.Mode == GroupModeNone {
		return true, nil
	}

	// First retrieve role commands for this group
	commands, err := rg.RoleCommands().AllG(ctx)
	if err != nil {
		return false, err
	}

	if rg.Mode == GroupModeSingle {
		// If user already has role it's attempting to give itself, assume were trying to remove it
		if common.ContainsInt64Slice(ms.Roles, targetRole.Role) {
			if rg.SingleRequireOne {
				return false, NewGroupError("Need at least one role in group **%s**", rg)
			}

			return true, nil
		}

		// Check if the user has any other role commands in this group
		for _, v := range commands {
			if common.ContainsInt64Slice(ms.Roles, v.Role) {
				if !rg.SingleAutoToggleOff {
					return false, NewGroupError("Max 1 role in group **%s** is allowed", rg)
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
		if common.ContainsInt64Slice(ms.Roles, role.Role) {
			hasRoles++
			if role.ID == targetRole.ID {
				hasTargetRole = true
			}
		}
	}

	if hasTargetRole {
		if hasRoles-1 < int(rg.MultipleMin) {
			err = NewLmitError("Minimum of `%d` roles required in this group", int(rg.MultipleMin))
			return false, err
		}
	} else {
		if hasRoles+1 > int(rg.MultipleMax) {
			err = NewLmitError("Maximum of `%d` roles allowed in this group", int(rg.MultipleMax))
			return false, err
		}
	}

	// If we got here then all checks passed
	return true, nil
}

// AssignRoleToMember attempts to assign the given role command, part of this group
// to the member
func GroupToggleRole(ctx context.Context, ms *dstate.MemberState, targetRole *models.RoleCommand) (gaveRole bool, err error) {
	rg := targetRole.R.RoleGroup
	guildID := targetRole.GuildID

	if can, err := GroupCanRole(ctx, ms, targetRole); !can {
		return false, err
	}

	// Default behaviour of groups is no more restrictions than reuiqred and ignore roles
	if rg.Mode != GroupModeSingle {
		// We already passed all checks
		gaveRole, err = ToggleRole(ms, targetRole.Role)
		if gaveRole && err == nil {
			err = GroupMaybeScheduleRoleRemoval(ctx, ms, targetRole)
		}
		return gaveRole, err
	}

	// If user already has role it's attempting to give itself
	if common.ContainsInt64Slice(ms.Roles, targetRole.Role) {
		err = common.BotSession.GuildMemberRoleRemove(guildID, ms.ID, targetRole.Role)
		return false, err
	}

	// Check if the user has any other role commands in this group
	for _, v := range rg.R.RoleCommands {
		if common.ContainsInt64Slice(ms.Roles, v.Role) {
			if rg.SingleAutoToggleOff {
				common.BotSession.GuildMemberRoleRemove(guildID, ms.ID, v.Role)
			} else {
				return false, NewGroupError("Max 1 role in group **%s** is allowed", rg)
			}
		}
	}

	// Finally give the role
	err = common.BotSession.GuildMemberRoleAdd(guildID, ms.ID, targetRole.Role)
	if err == nil {
		err = GroupMaybeScheduleRoleRemoval(ctx, ms, targetRole)
	}
	return true, err
}

func GroupMaybeScheduleRoleRemoval(ctx context.Context, ms *dstate.MemberState, targetRole *models.RoleCommand) error {
	temporaryDuration := targetRole.R.RoleGroup.TemporaryRoleDuration
	if temporaryDuration == 0 {
		return nil
	}

	// remove existing role removal events for this role
	_, err := schEvtsModels.ScheduledEvents(qm.Where("event_name='remove_member_role' AND  guild_id = ? AND (data->>'user_id')::bigint = ? AND (data->>'role_id')::bigint = ?", ms.Guild.ID, ms.ID, targetRole.Role)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return err
	}

	// add the scheduled event for it
	err = scheduledevents2.ScheduleEvent("remove_member_role", ms.Guild.ID, time.Now().Add(time.Duration(temporaryDuration)*time.Minute), &ScheduledMemberRoleRemoveData{
		GuildID: ms.Guild.ID,
		GroupID: targetRole.R.RoleGroup.ID,
		UserID:  ms.ID,
		RoleID:  targetRole.Role,
	})

	if err != nil {
		return err
	}

	return nil
}

func CanAssignRoleCmdTo(r *models.RoleCommand, memberRoles []int64) error {

	if len(r.RequireRoles) > 0 {
		if !CheckRequiredRoles(r.RequireRoles, memberRoles) {
			return NewSimpleError("This self assignable role has been configured to require another role by the server admins.")
		}
	}

	if len(r.IgnoreRoles) > 0 {
		if err := CheckIgnoredRoles(r.IgnoreRoles, memberRoles); err != nil {
			return err
		}
	}

	return nil
}

func CheckRequiredRoles(requireOneOf []int64, has []int64) bool {
	for _, r := range requireOneOf {
		if common.ContainsInt64Slice(has, r) {
			// Only 1 role required
			return true
		}
	}

	return false
}

func CheckIgnoredRoles(ignore []int64, has []int64) error {
	for _, r := range ignore {
		if common.ContainsInt64Slice(has, r) {
			return NewRoleError("Has ignored role", r)
		}
	}

	return nil
}

// Just a simple type but distinguishable from errors.Error
type SimpleError string

func (s SimpleError) Error() string {
	return string(s)
}

func NewSimpleError(format string, args ...interface{}) error {
	return SimpleError(fmt.Sprintf(format, args...))
}

type RoleError struct {
	Role    int64
	Message string
}

func NewRoleError(msg string, role int64) error {
	return &RoleError{
		Role:    role,
		Message: msg,
	}
}

func (r *RoleError) Error() string {
	if r.Role == 0 {
		return r.Message
	}
	return r.Message + ": " + strconv.FormatInt(r.Role, 10)
}

// Uses the role name from one of the passed roles with matching id instead of the id
func (r *RoleError) PrettyError(roles []*discordgo.Role) string {
	if r.Role == 0 {
		return r.Message
	}

	idStr := strconv.FormatInt(r.Role, 10)

	roleStr := ""

	for _, v := range roles {
		if v.ID == r.Role {
			roleStr = "**" + v.Name + "**"
			break
		}
	}

	if roleStr == "" {
		roleStr = "(unknown role " + idStr + ")"
	}

	return r.Message + ": " + roleStr
}

type LmitError struct {
	Limit   int
	Message string
}

func NewLmitError(msg string, limit int) error {
	return &LmitError{
		Limit:   limit,
		Message: msg,
	}
}

func (r *LmitError) Error() string {
	return fmt.Sprintf(r.Message, r.Limit)
}

type GroupError struct {
	Group   *models.RoleGroup
	Message string
}

func NewGroupError(msg string, group *models.RoleGroup) error {
	return &GroupError{
		Group:   group,
		Message: msg,
	}
}

func (r *GroupError) Error() string {
	return fmt.Sprintf(r.Message, r.Group.Name)
}

func IsRoleCommandError(err error) bool {
	switch err.(type) {
	case *LmitError, *RoleError, *GroupError, SimpleError, *SimpleError:
		return true
	default:
		return false
	}
}

func RoleCommandsLessFunc(slice []*models.RoleCommand) func(int, int) bool {
	return func(i, j int) bool {
		// Compare timestamps if positions are equal, for deterministic output
		if slice[i].Position == slice[j].Position {
			return slice[i].CreatedAt.After(slice[j].CreatedAt)
		}

		if slice[i].Position > slice[j].Position {
			return false
		}

		return true
	}
}

func GetAllRoleCommandsSorted(ctx context.Context, guildID int64) (groups []*models.RoleGroup, grouped map[*models.RoleGroup][]*models.RoleCommand, unGrouped []*models.RoleCommand, err error) {
	commands, err := models.RoleCommands(qm.Where(models.RoleCommandColumns.GuildID+"=?", guildID)).AllG(ctx)
	if err != nil {
		return
	}

	grps, err := models.RoleGroups(qm.Where(models.RoleGroupColumns.GuildID+"=?", guildID)).AllG(ctx)
	if err != nil {
		return
	}
	groups = grps

	grouped = make(map[*models.RoleGroup][]*models.RoleCommand)
	for _, group := range groups {

		grouped[group] = make([]*models.RoleCommand, 0, 10)

		for _, cmd := range commands {
			if cmd.RoleGroupID.Valid && cmd.RoleGroupID.Int64 == group.ID {
				grouped[group] = append(grouped[group], cmd)
			}
		}

		sort.Slice(grouped[group], RoleCommandsLessFunc(grouped[group]))
	}

	unGrouped = make([]*models.RoleCommand, 0, 10)
	for _, cmd := range commands {
		if !cmd.RoleGroupID.Valid {
			unGrouped = append(unGrouped, cmd)
		}
	}
	sort.Slice(unGrouped, RoleCommandsLessFunc(unGrouped))

	err = nil
	return
}
