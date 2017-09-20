package rolecommands

//go:generate kallax gen -e "kallax.go" -e "rolecommands.go" -e "web.go" -e "bot.go"  -e "legacy.go" -e "menu.go"

import (
	"gopkg.in/src-d/go-kallax.v1"
)

type RoleCommand struct {
	kallax.Model `table:"role_commands" pk:"id,autoincr"`
	kallax.Timestamps

	ID      int64
	GuildID int64

	Name         string
	Group        *RoleGroup `fk:",inverse"`
	Role         int64
	RequireRoles []int64
	IgnoreRoles  []int64

	Position int
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

type RoleMenuState int

const (
	RoleMenuStateSettingUp RoleMenuState = 0
	RoleMenuStateDone      RoleMenuState = 1
)

type RoleMenu struct {
	kallax.Model `table:"role_menus" pk:"message_id"`
	MessageID    int64
	ChannelID    int64
	GuildID      int64
	OwnerID      int64 // The user that created this, only this user can continue the set up process

	OwnMessage bool // Wether the menus is the bot's message, if so we will create and update the menu description aswell

	State           RoleMenuState
	NextRoleCommand *RoleCommand `fk:"next_role_command_id,inverse"`

	Group *RoleGroup `fk:",inverse"`

	Options []*RoleMenuOption
}

type RoleMenuOption struct {
	kallax.Model `table:"role_menu_options" pk:"id,autoincr"`
	ID           int64
	RoleCmd      *RoleCommand `fk:",inverse"`
	Menu         *RoleMenu    `fk:",inverse"`

	EmojiID      int64
	UnicodeEmoji string
}
