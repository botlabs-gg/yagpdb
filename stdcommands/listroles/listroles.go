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
	ArgSwitches: []*dcmd.ArgDef{
		{Switch: "nomanaged", Name: "Don't list managed/bot roles"},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		var out, outFinal string
		var noMana bool
		
		if data.Switches["nomanaged"].Value != nil && data.Switches["nomanaged"].Value.(bool) {
			noMana = true
		}
		
		data.GS.Lock()
		defer data.GS.Unlock()

		sort.Sort(dutil.Roles(data.GS.Guild.Roles))
		
		counter := 0
		for _, r := range data.GS.Guild.Roles {
			if noMana && r.Managed {
				continue
			} else {
				counter++
				me := r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0
				out += fmt.Sprintf("`%-25s: %-19d #%-6x  ME:%5t`\n", r.Name, r.ID, r.Color, me)
			}
		}
		outFinal = fmt.Sprintf("Total role count: %d\n", counter)
		outFinal += fmt.Sprintf("%s", "(ME = mention everyone perms)\n")
		outFinal += out
		return outFinal, nil
	},
}
