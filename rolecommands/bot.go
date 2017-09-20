package rolecommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"gopkg.in/src-d/go-kallax.v1"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(
		&commands.CustomCommand{
			Category: commands.CategoryTool,
			Command: &commandsystem.Command{
				Name:        "Role",
				Description: "Give yourself a role or list all available roles",
				Arguments: []*commandsystem.ArgDef{
					&commandsystem.ArgDef{Name: "Role", Type: commandsystem.ArgumentString},
				},
				Run: CmdFuncRole,
			},
		}, &commands.CustomCommand{
			Category: commands.CategoryTool,
			Command: &commandsystem.Command{
				Name:         "RoleMenu",
				Description:  "Set up a role menu",
				RequiredArgs: 1,
				Arguments: []*commandsystem.ArgDef{
					&commandsystem.ArgDef{Name: "Group", Type: commandsystem.ArgumentString},
				},
				Run: CmdFuncRoleMenu,
			},
		},
	)

	eventsystem.AddHandler(handleReactionAdd, eventsystem.EventMessageReactionAdd)
}

func CmdFuncRole(parsed *commandsystem.ExecData) (interface{}, error) {
	if parsed.Args[0] == nil {
		return CmdFuncListCommands(parsed)
	}

	member, err := bot.GetMember(parsed.Guild.ID(), parsed.Message.Author.ID)
	if err != nil {
		return "Failed retrieving you?", err
	}

	given, err := FindAssignRole(parsed.Guild.ID(), member, parsed.Args[0].Str())
	if err != nil {
		if err == kallax.ErrNotFound {
			resp, err := CmdFuncListCommands(parsed)
			if v, ok := resp.(string); ok {
				return "Role not round, " + v, err
			}

			return resp, err
		}

		return HumanizeAssignError(parsed.Guild, err), err
	}

	if given {
		return "Gave you the role!", nil
	}

	return "Took away your role!", nil
}

func HumanizeAssignError(guild *dstate.GuildState, err error) string {
	if IsRoleCommandError(err) {
		if roleError, ok := err.(*RoleError); ok {
			guild.RLock()
			defer guild.RUnlock()

			return roleError.PrettyError(guild.Guild.Roles)
		}
		return err.Error()
	}

	if code, _ := common.DiscordError(err); code != 0 {
		if code == discordgo.ErrCodeMissingPermissions {
			return "Bot does not have enough permissions to assign you this role"
		}
	}

	return "An error occured assignign the role"

}

func CmdFuncListCommands(parsed *commandsystem.ExecData) (interface{}, error) {
	_, grouped, ungrouped, err := GetAllRoleCommandsSorted(common.MustParseInt(parsed.Guild.ID()))
	if err != nil {
		return "Failed retrieving role commands", err
	}

	output := "Here is a list of available roles:\n"

	didListCommands := false
	for group, cmds := range grouped {
		if len(cmds) < 1 {
			continue
		}
		didListCommands = true

		output += "**" + group.Name + "**\n"
		output += StringCommands(cmds)
		output += "\n"
	}

	if len(ungrouped) > 0 {
		didListCommands = true

		output += "**Ungrouped roles**\n"
		output += StringCommands(ungrouped)
	}

	if !didListCommands {
		output += "No role commands (self assignable roles) set up. You can set them up in the control panel."
	}

	return output, nil
}

// StringCommands pretty formats a bunch of commands into  a string
func StringCommands(cmds []*RoleCommand) string {
	stringedCommands := make([]int64, 0, len(cmds))

	output := "```\n"

	for _, cmd := range cmds {
		if common.ContainsInt64Slice(stringedCommands, cmd.Role) {
			continue
		}

		output += cmd.Name
		// Check for duplicate roles
		for _, cmd2 := range cmds {
			if cmd.Role == cmd2.Role && cmd.Name != cmd2.Name {
				output += "/ " + cmd2.Name
			}
		}
		output += "\n"

		stringedCommands = append(stringedCommands, cmd.Role)
	}

	return output + "```\n"
}
