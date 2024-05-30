// rolecommands is a plugin which allows users to assign roles to themselves
package rolecommands

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/tidwall/buntdb"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
	cmd, err := models.RoleCommands(qm.Where("guild_id=?", ms.GuildID), qm.Where("name ILIKE ?", name), qm.Load("RoleGroup.RoleCommands")).OneG(ctx)
	if err != nil {
		return false, err
	}

	cr := CommonRoleFromRoleCommand(cmd.R.RoleGroup, cmd)
	return cr.CheckToggleRole(ctx, ms)
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
func (r *RoleError) PrettyError(roles []discordgo.Role) string {
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

type CommonRoleError struct {
	Group   *CommonRoleSettings
	Message string
}

func NewCommonRoleError(msg string, r *CommonRoleSettings) error {
	return &CommonRoleError{
		Group:   r,
		Message: msg,
	}
}

func (r *CommonRoleError) Error() string {
	name := ""
	if r.Group.ParentGroup != nil {
		name = r.Group.ParentGroup.Name
	} else {
		name = "role menu"
	}

	return fmt.Sprintf(r.Message, name)
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
