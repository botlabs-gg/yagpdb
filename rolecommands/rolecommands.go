// rolecommands is a plugin which allows users to assign roles to themselves
package rolecommands

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs"
	"github.com/jonas747/yagpdb/web"
	"gopkg.in/src-d/go-kallax.v1"
	"sort"
	"strconv"
)

//go:generate esc -o assets_gen.go -pkg rolecommands -ignore ".go" assets/

var (
	groupStore          *RoleGroupStore
	cmdStore            *RoleCommandStore
	roleMenuStore       *RoleMenuStore
	roleMenuOptionStore *RoleMenuOptionStore
)

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "RoleCommands"
}

var (
	_ common.Plugin = (*Plugin)(nil)
	_ web.Plugin    = (*Plugin)(nil)
	_ bot.Plugin    = (*Plugin)(nil)
)

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)

	_, err := common.PQ.Exec(FSMustString(false, "/assets/schema.sql"))
	if err != nil {
		logrus.WithError(err).Fatal("Failed initializing db schema")
	}

	groupStore = NewRoleGroupStore(common.PQ)
	cmdStore = NewRoleCommandStore(common.PQ)
	roleMenuStore = NewRoleMenuStore(common.PQ)
	roleMenuOptionStore = NewRoleMenuOptionStore(common.PQ)

	docs.AddPage("Role Commands / Self assignable roles", FSMustString(false, "/assets/help.md"), nil)
}

func FindAssignRole(guildID string, member *discordgo.Member, name string) (gaveRole bool, err error) {
	parsedGuildID := common.MustParseInt(guildID)
	cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByGuildID(kallax.Eq, parsedGuildID).Where(kallax.Ilike(Schema.RoleCommand.Name, name)).WithGroup())
	if err != nil {
		return false, err
	}

	return AssignRole(parsedGuildID, member, cmd)
}

// AssignRole attempts to assign the given role command, returns an error if the role does not exists
// or is unable to receie said role
func AssignRole(guildID int64, member *discordgo.Member, cmd *RoleCommand) (gaveRole bool, err error) {
	// We work with int64's internally
	parsedRoles := make([]int64, len(member.Roles))
	for i, v := range member.Roles {
		parsedRoles[i], _ = strconv.ParseInt(v, 10, 64)
	}

	if err := cmd.CanAssignTo(parsedRoles); err != nil {
		return false, err
	}

	// This command belongs to a group, let the group handle it
	if cmd.Group != nil {
		return cmd.Group.AssignRoleToMember(guildID, member, parsedRoles, cmd)
	}

	// This is a single command, just toggle it
	return ToggleRole(guildID, member, parsedRoles, cmd.Role)
}

// ToggleRole toggles the role of a guildmember, adding it if the member does not have the role and removing it if they do
func ToggleRole(guildID int64, member *discordgo.Member, parsedMemberRoles []int64, role int64) (gaveRole bool, err error) {
	if common.ContainsInt64Slice(parsedMemberRoles, role) {
		err = common.BotSession.GuildMemberRoleRemove(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(role, 10))
		gaveRole = false
		return
	}

	err = common.BotSession.GuildMemberRoleAdd(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(role, 10))
	gaveRole = true
	return
}

// AssignRoleToMember attempts to assign the given role command, part of this group
// to the member
func (rg *RoleGroup) AssignRoleToMember(guildID int64, member *discordgo.Member, parsedRoles []int64, targetRole *RoleCommand) (gaveRole bool, err error) {
	if len(rg.RequireRoles) > 0 {
		if !CheckRequiredRoles(rg.RequireRoles, parsedRoles) {
			err = NewSimpleError("Missing a required role")
			return
		}
	}

	if len(rg.IgnoreRoles) > 0 {
		if err = CheckIgnoredRoles(rg.IgnoreRoles, parsedRoles); err != nil {
			return
		}
	}

	// Default behaviour of groups is no more restrictions than reuiqred and ignore roles
	if rg.Mode == GroupModeNone {
		return ToggleRole(guildID, member, parsedRoles, targetRole.Role)
	}

	// First retrieve role commands for this group
	commands, err := cmdStore.FindAll(NewRoleCommandQuery().FindByGroup(rg.ID))
	if err != nil {
		return
	}

	if rg.Mode == GroupModeSingle {
		// If user already has role it's attempting to give itself
		if common.ContainsInt64Slice(parsedRoles, targetRole.Role) {
			if rg.SingleRequireOne {
				return false, NewGroupError("Need atleast one role in group **%s**", rg)
			}
			err = common.BotSession.GuildMemberRoleRemove(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(targetRole.Role, 10))
			gaveRole = false
			return
		}

		// Check if the user has any other role commands in this group
		for _, v := range commands {
			if common.ContainsInt64Slice(parsedRoles, v.Role) {
				if rg.SingleAutoToggleOff {
					common.BotSession.GuildMemberRoleRemove(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(v.Role, 10))
				} else {
					return false, NewGroupError("Max 1 role in group **%s** is allowed", rg)
				}
			}
		}
		// Finally give the role
		err = common.BotSession.GuildMemberRoleAdd(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(targetRole.Role, 10))
		return true, err
	}

	// Handle multiple mode

	// Count roles in group and check against min-max
	// also check if we already have said role
	hasRoles := 0
	hasTargetRole := false
	for _, role := range commands {
		if common.ContainsInt64Slice(parsedRoles, role.Role) {
			hasRoles++
			if role.ID == targetRole.ID {
				hasTargetRole = true
			}
		}
	}

	if hasTargetRole {
		if hasRoles-1 < rg.MultipleMin {
			err = NewLmitError("Minimum of `%d` roles required in this group", rg.MultipleMin)
			return
		}
	} else {
		if hasRoles+1 > rg.MultipleMax {
			err = NewLmitError("Maximum of `%d` roles allowed in this group", rg.MultipleMax)
			return
		}
	}

	return ToggleRole(guildID, member, parsedRoles, targetRole.Role)
}

func (r *RoleCommand) CanAssignTo(memberRoles []int64) error {

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
		if v.ID == idStr {
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
	Group   *RoleGroup
	Message string
}

func NewGroupError(msg string, group *RoleGroup) error {
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

func RoleCommandsLessFunc(slice []*RoleCommand) func(int, int) bool {
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

func GetAllRoleCommandsSorted(guildID int64) (groups []*RoleGroup, grouped map[*RoleGroup][]*RoleCommand, unGrouped []*RoleCommand, err error) {
	commands, err := cmdStore.FindAll(NewRoleCommandQuery().WithGroup().FindByGuildID(kallax.Eq, guildID))
	if err != nil && err != kallax.ErrNotFound {
		return
	}

	groups, err = groupStore.FindAll(NewRoleGroupQuery().FindByGuildID(kallax.Eq, guildID))
	if err != nil && err != kallax.ErrNotFound {
		return
	}

	grouped = make(map[*RoleGroup][]*RoleCommand)
	for _, group := range groups {
		grouped[group] = make([]*RoleCommand, 0, 10)
		for _, cmd := range commands {
			if cmd.Group != nil && cmd.Group.ID == group.ID {
				grouped[group] = append(grouped[group], cmd)
			}
		}

		sort.Slice(grouped[group], RoleCommandsLessFunc(grouped[group]))
	}

	unGrouped = make([]*RoleCommand, 0, 10)
	for _, cmd := range commands {
		if cmd.Group == nil {
			unGrouped = append(unGrouped, cmd)
		}
	}
	sort.Slice(unGrouped, RoleCommandsLessFunc(unGrouped))

	err = nil
	return
}

func kallaxDebugger(message string, args ...interface{}) {
	logrus.Debugf("%s, args: %v", message, args)
}
