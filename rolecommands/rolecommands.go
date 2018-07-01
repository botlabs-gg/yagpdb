// rolecommands is a plugin which allows users to assign roles to themselves
package rolecommands

//go:generate sqlboiler --no-hooks -w "role_groups,role_commands,role_menus,role_menu_options" postgres

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/rolecommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"sort"
	"strconv"
	"time"
)

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "RoleCommands"
}

const (
	GroupModeNone = iota
	GroupModeSingle
	GroupModeMultiple
)

const (
	RoleMenuStateSettingUp = 0
	RoleMenuStateDone      = 1
)

var (
	_ common.Plugin = (*Plugin)(nil)
	_ web.Plugin    = (*Plugin)(nil)
	_ bot.Plugin    = (*Plugin)(nil)

	cooldownsDB *buntdb.DB
)

func RegisterPlugin() {
	cooldownsDB, _ = buntdb.Open(":memory:")

	p := &Plugin{}
	common.RegisterPlugin(p)

	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Fatal("Failed initializing db schema")
	}
}

type CommandGroupPair struct {
	Command *models.RoleCommand
	Group   *models.RoleGroup
}

func FindAssignRole(guildID int64, member *discordgo.Member, name string) (gaveRole bool, err error) {
	cmd, err := models.RoleCommandsG(qm.Where("guild_id=?", guildID), qm.Where("name ILIKE ?", name)).One()
	if err != nil {
		return false, err
	}
	var group *models.RoleGroup
	if cmd.RoleGroupID.Valid {
		group, err = cmd.RoleGroupG().One()
		if err != nil {
			return false, err
		}
	}

	return AssignRole(guildID, member, &CommandGroupPair{Command: cmd, Group: group})
}

// AssignRole attempts to assign the given role command, returns an error if the role does not exists
// or is unable to receie said role
func AssignRole(guildID int64, member *discordgo.Member, cmd *CommandGroupPair) (gaveRole bool, err error) {
	onCD := false

	// First check cooldown
	err = cooldownsDB.Update(func(tx *buntdb.Tx) error {
		_, replaced, _ := tx.Set(discordgo.StrID(member.User.ID), "1", &buntdb.SetOptions{Expires: true, TTL: time.Second * 1})
		if replaced {
			onCD = true
		}
		return nil
	})

	if onCD {
		return false, NewSimpleError("You're on cooldown")
	}

	if err := CanAssignRoleCmdTo(cmd.Command, member.Roles); err != nil {
		return false, err
	}

	// This command belongs to a group, let the group handle it
	if cmd.Group != nil {
		return GroupAssignRoleToMember(cmd.Group, guildID, member, cmd.Command)
	}

	// This is a single command, just toggle it
	return ToggleRole(guildID, member, cmd.Command.Role)
}

// ToggleRole toggles the role of a guildmember, adding it if the member does not have the role and removing it if they do
func ToggleRole(guildID int64, member *discordgo.Member, role int64) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(member.Roles, role) {
		err = common.BotSession.GuildMemberRoleRemove(guildID, member.User.ID, role)
		gaveRole = false
		return
	}

	err = common.BotSession.GuildMemberRoleAdd(guildID, member.User.ID, role)
	gaveRole = true
	return
}

// AssignRoleToMember attempts to assign the given role command, part of this group
// to the member
func GroupAssignRoleToMember(rg *models.RoleGroup, guildID int64, member *discordgo.Member, targetRole *models.RoleCommand) (gaveRole bool, err error) {
	if len(rg.RequireRoles) > 0 {
		if !CheckRequiredRoles(rg.RequireRoles, member.Roles) {
			err = NewSimpleError("Missing a required role")
			return
		}
	}

	if len(rg.IgnoreRoles) > 0 {
		if err = CheckIgnoredRoles(rg.IgnoreRoles, member.Roles); err != nil {
			return
		}
	}

	// Default behaviour of groups is no more restrictions than reuiqred and ignore roles
	if rg.Mode == GroupModeNone {
		return ToggleRole(guildID, member, targetRole.Role)
	}

	// First retrieve role commands for this group
	commands, err := rg.RoleCommandsG().All()
	if err != nil {
		return
	}

	if rg.Mode == GroupModeSingle {
		// If user already has role it's attempting to give itself
		if common.ContainsInt64Slice(member.Roles, targetRole.Role) {
			if rg.SingleRequireOne {
				return false, NewGroupError("Need atleast one role in group **%s**", rg)
			}
			err = common.BotSession.GuildMemberRoleRemove(guildID, member.User.ID, targetRole.Role)
			gaveRole = false
			return
		}

		// Check if the user has any other role commands in this group
		for _, v := range commands {
			if common.ContainsInt64Slice(member.Roles, v.Role) {
				if rg.SingleAutoToggleOff {
					common.BotSession.GuildMemberRoleRemove(guildID, member.User.ID, v.Role)
				} else {
					return false, NewGroupError("Max 1 role in group **%s** is allowed", rg)
				}
			}
		}
		// Finally give the role
		err = common.BotSession.GuildMemberRoleAdd(guildID, member.User.ID, targetRole.Role)
		return true, err
	}

	// Handle multiple mode

	// Count roles in group and check against min-max
	// also check if we already have said role
	hasRoles := 0
	hasTargetRole := false
	for _, role := range commands {
		if common.ContainsInt64Slice(member.Roles, role.Role) {
			hasRoles++
			if role.ID == targetRole.ID {
				hasTargetRole = true
			}
		}
	}

	if hasTargetRole {
		if hasRoles-1 < int(rg.MultipleMin) {
			err = NewLmitError("Minimum of `%d` roles required in this group", int(rg.MultipleMin))
			return
		}
	} else {
		if hasRoles+1 > int(rg.MultipleMax) {
			err = NewLmitError("Maximum of `%d` roles allowed in this group", int(rg.MultipleMax))
			return
		}
	}

	return ToggleRole(guildID, member, targetRole.Role)
}

func CanAssignRoleCmdTo(r *models.RoleCommand, memberRoles []int64) error {

	if len(r.RequireRoles) > 0 {
		if !CheckRequiredRoles(r.RequireRoles, memberRoles) {
			return NewSimpleError("Missing a required role")
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

func GetAllRoleCommandsSorted(guildID int64) (groups []*models.RoleGroup, grouped map[*models.RoleGroup][]*models.RoleCommand, unGrouped []*models.RoleCommand, err error) {
	commands, err := models.RoleCommandsG(qm.Where(models.RoleCommandColumns.GuildID+"=?", guildID)).All()
	if err != nil {
		return
	}

	grps, err := models.RoleGroupsG(qm.Where(models.RoleGroupColumns.GuildID+"=?", guildID)).All()
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
