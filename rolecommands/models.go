package rolecommands

//go:generate kallax gen -e "kallax.go" -e "rolecommands.go" -e "web.go"

import (
	"gopkg.in/src-d/go-kallax.v1"
)

type RoleCommand struct {
	kallax.Model `table:"role_commands" pk:"id,autoincr"`
	ID           int64
	GuildID      int64

	Name         string
	Group        *RoleGroup `fk:",inverse"`
	Role         int64
	RequireRoles []int64
	IgnoreRoles  []int64
}

func newRoleCommand() *RoleCommand {
	return &RoleCommand{
		RequireRoles: []int64{},
		IgnoreRoles:  []int64{},
	}
}

type GroupMode int

const (
	GroupModeNone GroupMode = iota
	GroupModeSingle
	GroupModeMultiple
)

type RoleGroup struct {
	kallax.Model `table:"role_groups" pk:"id,autoincr"`
	ID           int64
	GuildID      int64

	Name         string
	RequireRoles []int64
	IgnoreRoles  []int64
	Mode         GroupMode

	MultipleMax int
	MultipleMin int

	SingleAutoToggleOff bool
	SingleRequireOne    bool
}
