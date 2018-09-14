package listroles

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/commands"
	"sort"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "ListRoles",
	Description: "List roles and their id's, and some other stuff on the server",

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
