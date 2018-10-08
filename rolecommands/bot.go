package rolecommands

import (
	"database/sql"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/rolecommands/models"
)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(
		&commands.YAGCommand{
			CmdCategory: commands.CategoryTool,
			Name:        "Role",
			Description: "Give yourself a role or list all available roles",
			Arguments: []*dcmd.ArgDef{
				&dcmd.ArgDef{Name: "Role", Type: dcmd.String},
			},
			RunFunc: CmdFuncRole,
		}, &commands.YAGCommand{
			CmdCategory: commands.CategoryTool,
			Name:        "RoleMenu",
			Description: "Set up a role menu, specify a message with -m to use an existing message instead of having the bot make one, if there's already a menu on this message it will be updated with the missing options.",
			Arguments: []*dcmd.ArgDef{
				&dcmd.ArgDef{Name: "Group", Type: dcmd.String},
			},
			ArgSwitches: []*dcmd.ArgDef{
				&dcmd.ArgDef{Switch: "m", Name: "Message ID", Type: &dcmd.IntArg{}},
				&dcmd.ArgDef{Switch: "nodm", Name: "Disable DM"},
				&dcmd.ArgDef{Switch: "rr", Name: "Remove role on reaction removed"},
			},
			RunFunc: CmdFuncRoleMenu,
		},
	)
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(handleReactionAddRemove, eventsystem.EventMessageReactionAdd, eventsystem.EventMessageReactionRemove)
	eventsystem.AddHandler(handleMessageRemove, eventsystem.EventMessageDelete, eventsystem.EventMessageDeleteBulk)
}

func CmdFuncRole(parsed *dcmd.Data) (interface{}, error) {
	if parsed.Args[0].Value == nil {
		return CmdFuncListCommands(parsed)
	}

	member, err := bot.GetMember(parsed.GS.ID, parsed.Msg.Author.ID)
	if err != nil {
		return nil, err
	}

	given, err := FindToggleRole(parsed.Context(), member, parsed.Args[0].Str())
	if err != nil {
		if err == sql.ErrNoRows {
			resp, err := CmdFuncListCommands(parsed)
			if v, ok := resp.(string); ok {
				return "Role not found, " + v, err
			}

			return resp, err
		}

		return HumanizeAssignError(parsed.GS, err)
	}

	if given {
		return "Gave you the role!", nil
	}

	return "Took away your role!", nil
}

func HumanizeAssignError(guild *dstate.GuildState, err error) (string, error) {
	if IsRoleCommandError(err) {
		if roleError, ok := err.(*RoleError); ok {
			guild.RLock()
			defer guild.RUnlock()

			return roleError.PrettyError(guild.Guild.Roles), nil
		}
		return err.Error(), nil
	}

	if code, msg := common.DiscordError(err); code != 0 {
		if code == discordgo.ErrCodeMissingPermissions {
			return "The bot is below the role, contact the server admin", err
		} else if code == discordgo.ErrCodeMissingAccess {
			return "Bot does not have enough permissions to assign you this role, contact the server admin", err
		}

		return "An error occured while assigning the role: " + msg, err
	}

	return "An error occurred while assigning the role", err

}

func CmdFuncListCommands(parsed *dcmd.Data) (interface{}, error) {
	_, grouped, ungrouped, err := GetAllRoleCommandsSorted(parsed.Context(), parsed.GS.ID)
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
func StringCommands(cmds []*models.RoleCommand) string {
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
