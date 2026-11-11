package listroles

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// role names are shown as they are, not as markdown
var markdownEscaper = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "~", `\~`, "`", "\\`", "|", `\|`, ">", `\>`)

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
		noManaged := data.Switches["nomanaged"].Value != nil && data.Switches["nomanaged"].Value.(bool)

		var lines []string
		for _, r := range data.GuildData.GS.Roles {
			if noManaged && r.Managed {
				continue
			}

			lines = append(lines, roleLine(&r))
		}

		return fmt.Sprintf("Total role count: %d\n(ME = mention everyone perms)\n%s", len(lines), strings.Join(lines, "\n")), nil
	},
}

// roleLine is a line per role rather than a table, which breaks on long names once the line wraps
func roleLine(r *discordgo.Role) string {
	// the counts of all roles are fetched together and cached, so this is one request for the whole list
	members := "?"
	if count, err := r.MemberCount(); err == nil {
		members = strconv.Itoa(count)
	}

	line := fmt.Sprintf("**%s** · %s members · `%d` · #%06x", markdownEscaper.Replace(r.Name), members, r.ID, r.Color)
	if r.Permissions&discordgo.PermissionAdministrator != 0 || r.Permissions&discordgo.PermissionMentionEveryone != 0 {
		line += " · ME"
	}

	return line
}
