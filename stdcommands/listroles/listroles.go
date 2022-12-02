package listroles

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "ListRoles",
	Description: "List roles, their id's, color hex code, and 'mention everyone' perms (useful if you wanna double check to make sure you didn't give anyone mention everyone perms that shouldn't have it)",
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "nomanaged", Help: "Don't list managed/bot roles"},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var out, outFinal string
		var noMana bool

		if data.Switches["nomanaged"].Value != nil && data.Switches["nomanaged"].Value.(bool) {
			noMana = true
		}

		// sort.Sort(dutil.Roles(data.GuildData.GS.Roles))

		counter := 0
		for _, r := range data.GuildData.GS.Roles {
			if noMana && r.Managed {
				continue
			} else {
				counter++
				me := r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0
				out += fmt.Sprintf("`%-25s: %-19d #%-6x  ME:%5t`\n", r.Name, r.ID, r.Color, me)
			}
		}
		outFinal = fmt.Sprintf("Total role count: %d\n", counter)
		outFinal += "(ME = mention everyone perms)\n"
		outFinal += out
		return outFinal, nil
	},
}
