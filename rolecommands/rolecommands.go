// rolecommands is a plugin which allows users to assign roles to themselves
package rolecommands

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"strconv"
	"strings"
)

//go:generate esc -o assets_gen.go -pkg rolecommands -ignore ".go" assets/

var (
	ErrOneRequiredGroup = NewRoleError("Need atleast one role in this group", 0)
	ErrMaxOneRole       = NewRoleError("Max 1 role in this group is allowed", 0)
	ErrCantRemove       = NewRoleError("Cannot remove this role", 0)

	groupStore *RoleGroupStore
	cmdStore   *RoleCommandStore
)

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "RoleCommands"
}

var (
	_ common.Plugin = (*Plugin)(nil)
	_ web.Plugin    = (*Plugin)(nil)
)

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)

	_, err := common.PQ.Exec(FSMustString(false, "/assets/migrations/1502731768_initial.up.sql"))
	if err != nil {
		logrus.WithError(err).Fatal("Failed initializing db schema")
	}

}

// AssignRole attempts to assign the given role command, returns an error if the role does not exists
// or is unable to receie said role
func AssignRole(guildID string, member *discordgo.Member, name string) (gaveRole bool, err error) {
	// We word with int64's internally
	parsedGuildID := common.MustParseInt(guildID)
	parsedRoles := make([]int64, len(member.Roles))
	for i, v := range member.Roles {
		parsedRoles[i], _ = strconv.ParseInt(v, 10, 64)
	}
	cmd, err := cmdStore.FindOne(NewRoleCommandQuery().FindByNames(strings.ToLower(name)).WithGroup())
	if err != nil {
		return false, err
	}

	if err := cmd.CanAssignTo(parsedRoles); err != nil {
		return false, err
	}

	// This command belongs to a group, let the group handle it
	if cmd.Group != nil {
		return cmd.Group.AssignRoleToMember(parsedGuildID, member, parsedRoles, cmd)
	}

	// This is a single command, just toggle it
	return ToggleRole(parsedGuildID, member, parsedRoles, cmd.Role)
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
		if err = CheckRequiredRoles(rg.RequireRoles, parsedRoles); err != nil {
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
				return false, ErrOneRequiredGroup
			}
			err = common.BotSession.GuildMemberRoleRemove(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(targetRole.Role, 10))
			gaveRole = false
			return
		}

		// Check if the user has any other commands in this group
		for _, v := range commands {
			if common.ContainsInt64Slice(parsedRoles, v.ID) {
				if rg.SingleAutoToggleOff {
					common.BotSession.GuildMemberRoleRemove(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(v.Role, 10))
				} else {
					return false, ErrMaxOneRole
				}
			}
		}
		// Finally give the role
		err = common.BotSession.GuildMemberRoleAdd(strconv.FormatInt(guildID, 10), member.User.ID, strconv.FormatInt(targetRole.Role, 10))
		return
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
			err = NewLmitError("Minimum of %d roles required in this group", rg.MultipleMin)
			return
		}
	} else {
		if hasRoles+1 > rg.MultipleMax {
			err = NewLmitError("Maximum of %d roles allowed in this group", rg.MultipleMax)
			return
		}
	}

	return ToggleRole(guildID, member, parsedRoles, targetRole.Role)
}

func CheckRequiredRoles(requireOneOf []int64, has []int64) error {
	for _, r := range requireOneOf {
		if common.ContainsInt64Slice(has, r) {
			return NewRoleError("Missing required role", r)
		}
	}

	return nil
}

func CheckIgnoredRoles(ignore []int64, has []int64) error {
	for _, r := range ignore {
		if common.ContainsInt64Slice(has, r) {
			return NewRoleError("Has ignored role", r)
		}
	}

	return nil
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

func IsRoleCommandError(err error) bool {
	switch err.(type) {
	case *LmitError, *RoleError:
		return true
	default:
		return false
	}
}
