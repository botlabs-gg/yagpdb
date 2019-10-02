package automod

import (
	"context"
	"fmt"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
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

			data.GS.UserCacheDel(CacheKeyRulesets)
			data.GS.UserCacheDel(CacheKeyLists)

			enabledStr := "enabled"
			if !ruleset.Enabled {
				enabledStr = "disabled"
			}

			return fmt.Sprintf("Ruleset **%s** is now `%s`", ruleset.Name, enabledStr), nil
		},
	}

	cmdViewRulesets := &commands.YAGCommand{
		Name:                "Rulesets",
		Aliases:             []string{"r", "list", "l"},
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

				out.WriteString(fmt.Sprintf("%s: %s\n", v.Name, onOff))
			}
			out.WriteString("```")

			return out.String(), nil
		},
	}

	cmdLogs := &commands.YAGCommand{
		Name:        "Logs",
		Aliases:     []string{"log"},
		CmdCategory: commands.CategoryModeration,
		Description: "Shows the log of the last triggered automod rules, optionally filtering by user",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Page", Type: &dcmd.IntArg{Max: 10000}, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "user", Type: dcmd.UserID},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: paginatedmessages.PaginatedCommand(0, func(data *dcmd.Data, p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			skip := (page - 1) * 15
			userID := data.Switch("user").Int64()

			qms := []qm.QueryMod{qm.Where("guild_id=?", data.GS.ID), qm.OrderBy("id desc"), qm.Limit(15)}
			if skip != 0 {
				qms = append(qms, qm.Offset(skip))
			}

			if userID != 0 {
				qms = append(qms, qm.Where("user_id = ? ", userID))
			}

			entries, err := models.AutomodTriggeredRules(qms...).AllG(context.Background())
			if err != nil {
				return nil, err
			}

			if len(entries) < 1 && page > 1 {
				return nil, paginatedmessages.ErrNoResults
			}

			out := &strings.Builder{}
			out.WriteString("```\n")

			for _, v := range entries {
				t := v.CreatedAt.UTC().Format("02 Jan 2006 15:04")
				out.WriteString(fmt.Sprintf("[%-17s] - %s\nRS:%s - R:%s - TR:%s\n\n", t, v.UserName, v.RulesetName, v.RuleName, RulePartMap[v.TriggerTypeid].Name()))
			}
			out.WriteString("``` **RS** = ruleset, **R** = rule, **TR** = trigger")

			return &discordgo.MessageEmbed{
				Title:       "Automod logs",
				Description: out.String(),
			}, nil
		}),
	}

	container := commands.CommandSystem.Root.Sub("automod", "amod")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")

	container.AddCommand(cmdViewRulesets, cmdViewRulesets.GetTrigger())
	container.AddCommand(cmdToggleRuleset, cmdToggleRuleset.GetTrigger())
	container.AddCommand(cmdLogs, cmdLogs.GetTrigger())
}
