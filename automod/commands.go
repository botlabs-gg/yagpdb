package automod

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strings"
)

func (p *Plugin) AddCommands() {

	cmdToggleRuleset := &commands.YAGCommand{
		Name:         "Toggle",
		Aliases:      []string{"t"},
		CmdCategory:  commands.CategoryModeration,
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "ruleset name", Type: dcmd.String},
		},
		Description:         "Toggles a ruleset on/off",
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			rulesetName := data.Args[0].Str()
			ruleset, err := models.AutomodRulesets(qm.Where("guild_id = ? AND name ILIKE ?", data.GS.ID, rulesetName)).OneG(data.Context())
			if err != nil {
				return "Unable to fine the ruleset, did you type the name correctly?", err
			}

			ruleset.Enabled = !ruleset.Enabled
			_, err = ruleset.UpdateG(data.Context(), boil.Whitelist("enabled"))
			if err != nil {
				return nil, err
			}

			data.GS.UserCacheDel(true, CacheKeyRulesets)
			data.GS.UserCacheDel(true, CacheKeyLists)

			enabledStr := "enabled"
			if !ruleset.Enabled {
				enabledStr = "disabled"
			}

			return fmt.Sprintf("Ruleset **%s** is now `%s`", ruleset.Name, enabledStr), nil
		},
	}

	cmdViewRulesets := &commands.YAGCommand{
		Name:                "rulesets",
		Aliases:             []string{"r"},
		CmdCategory:         commands.CategoryModeration,
		Description:         "Lists all rulesets and their status",
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", data.GS.ID), qm.OrderBy("id asc")).AllG(data.Context())
			if err != nil {
				return nil, err
			}

			if len(rulesets) < 1 {
				return "No automod v2 rulesets set up on this server", nil
			}

			out := &strings.Builder{}
			out.WriteString("Rulesets on this server:\n```\n")
			for _, v := range rulesets {
				onOff := "Enabled"
				if !v.Enabled {
					onOff = "Disabled"
				}

				out.WriteString(fmt.Sprintf("%s: %s", v.Name, onOff))
			}
			out.WriteString("\n```")

			return out.String(), nil
		},
	}

	container := commands.CommandSystem.Root.Sub("automod")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")

	container.AddCommand(cmdViewRulesets, cmdViewRulesets.GetTrigger())
	container.AddCommand(cmdToggleRuleset, cmdToggleRuleset.GetTrigger())
}
