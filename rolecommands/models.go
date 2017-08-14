package rolecommands

//go:generate kallax gen -e "kallax.go" -e "rolecommands.go" -e "web.go"

import (
	"fmt"
	"github.com/jonas747/yagpdb/common"
	"gopkg.in/src-d/go-kallax.v1"
	"strconv"
)

type RoleCommand struct {
	kallax.Model `table:"role_commands" pk:"id,autoincr"`
	ID           int64
	GuildID      int64

	Names        []string
	Group        *RoleGroup `fk:",inverse"`
	Role         int64
	RequireRoles []int64
	IgnoreRoles  []int64
}

func (r *RoleCommand) CanAssignTo(memberRoles []int64) error {

	if len(r.RequireRoles) > 0 {
		if err := CheckRequiredRoles(r.RequireRoles, memberRoles); err != nil {
			return err
		}
	}

	if len(r.IgnoreRoles) > 0 {
		if err := CheckIgnoredRoles(r.IgnoreRoles, memberRoles); err != nil {
			return err
		}
	}

	return nil
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
