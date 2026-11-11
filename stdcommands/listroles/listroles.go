package listroles

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var Command = &commands.YAGCommand{
	CmdCategory:         commands.CategoryTool,
	Name:                "ListRoles",
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	Description:         "List roles, their id's, member count, color hex code, and 'mention everyone' perms (useful if you wanna double check to make sure you didn't give anyone mention everyone perms that shouldn't have it)",
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "nomanaged", Help: "Don't list managed/bot roles"},
	},

	RunFunc: func(data *dcmd.Data) (any, error) {
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
				out += roleLine(&r)
			}
		}
		outFinal = fmt.Sprintf("Total role count: %d\n", counter)
		outFinal += "(ME = mention everyone perms)\n"
		outFinal += out
		return outFinal, nil
	},
}

func roleLine(r *discordgo.Role) string {
	// the counts of all roles are fetched together and cached, so this is one request for the whole list
	members := "?"
	if count, err := r.MemberCount(); err == nil {
		members = strconv.Itoa(count)
	}

	// a backtick in the name would end the code span early
	name := strings.ReplaceAll(r.Name, "`", "'")
	me := r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0
	return fmt.Sprintf("`%-25s: %-19d #%-6x  Members:%-6s ME:%5t`\n", name, r.ID, r.Color, members, me)
}
