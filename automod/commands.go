package automod

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (p *Plugin) AddCommands() {

	cmdToggleRuleset := &commands.YAGCommand{
		Name:         "toggle",
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
				return "Unable to find the ruleset, did you type the name correctly?", err
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

	container := commands.CommandSystem.Root.Sub("automod")
	container.NotFound = func(data *dcmd.Data) (interface{}, error) {
		resp := dcmd.GenerateHelp(data, container, &dcmd.StdHelpFormatter{})
		if len(resp) > 0 {
			return resp[0], nil
		}

		return "Unknown automod command", nil
	}

	container.AddCommand(cmdToggleRuleset, cmdToggleRuleset.GetTrigger())
}
