package listroles

import (
	"fmt"
	"sort"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "ListRoles",
	Description: "List roles, their id's, color hex code, and 'mention everyone' perms (useful if you wanna double check to make sure you didn't give anyone mention everyone perms that shouldn't have it)",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		out := "(ME = mention everyone perms)\n"

		data.GS.Lock()
		defer data.GS.Unlock()

		sort.Sort(dutil.Roles(data.GS.Guild.Roles))

		for _, r := range data.GS.Guild.Roles {
			me := r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0
			out += fmt.Sprintf("`%-25s: %-19d #%-6x  ME:%5t`\n", r.Name, r.ID, r.Color, me)
		}

		return out, nil
	},
}
