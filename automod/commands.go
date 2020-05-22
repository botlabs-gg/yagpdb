package automod

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common/featureflags"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
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
			featureflags.MarkGuildDirty(data.GS.ID)

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

			if len(entries) < 1 && p != nil && p.LastResponse != nil { //Dont send No Results error on first execution
				return nil, paginatedmessages.ErrNoResults
			}

			out := &strings.Builder{}
			out.WriteString("```\n")

			if len(entries) > 0 {
				for _, v := range entries {
					t := v.CreatedAt.UTC().Format("02 Jan 2006 15:04")
					out.WriteString(fmt.Sprintf("[%-17s] - %s\nRS:%s - R:%s - TR:%s\n\n", t, v.UserName, v.RulesetName, v.RuleName, RulePartMap[v.TriggerTypeid].Name()))
				}
			} else {
				out.WriteString("No Entries")
			}
			out.WriteString("``` **RS** = ruleset, **R** = rule, **TR** = trigger")

			return &discordgo.MessageEmbed{
				Title:       "Automod logs",
				Description: out.String(),
			}, nil
		}),
	}

	cmdListVLC := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ListViolationsCount",
		Description:   "Lists Violations summary in entire server or of specified user optionally filtered by max violation age.\n Specify number of violations to skip while fetching using -skip flag ; max entries fetched 500",
		Aliases:       []string{"ViolationsCount", "VCount"},
		RequiredArgs:  0,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "ma", Name: "Max Violation Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Switch: "skip", Name: "Amount Skipped", Type: dcmd.Int, Default: 0},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers, discordgo.PermissionKickMembers, discordgo.PermissionManageMessages},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			userID := parsed.Args[0].Int64()
			order := "id desc"
			limit := 500

			//Check Flags
			maxAge := parsed.Switches["ma"].Value.(time.Duration)
			skip := parsed.Switches["skip"].Int()
			if skip < 0 {
				skip = 0
			}

			// retrieve Violations
			qms := []qm.QueryMod{qm.Where("guild_id = ?", parsed.GS.ID), qm.OrderBy(order), qm.Limit(limit), qm.Offset(skip)}

			if userID != 0 {
				qms = append(qms, qm.Where("user_id = ?", userID))
			}

			if maxAge != 0 {
				qms = append(qms, qm.Where("created_at > ?", time.Now().Add(-maxAge)))
			}

			listViolations, err := models.AutomodViolations(qms...).AllG(context.Background())

			if err != nil {
				return nil, err
			}

			if len(listViolations) < 1 {
				return "No Active Violations or No Violations fetched with specified conditions", nil
			}

			out := ""

			violations := make(map[string]int)
			for _, entry := range listViolations {
				violations[entry.Name] = violations[entry.Name] + 1
			}

			for name, count := range violations {
				out += fmt.Sprintf("Violation: %-20s Count: %d\n", name, count)
			}

			if out == "" {
				return "No Violations found with specified conditions", nil
			}

			out = "```" + out + fmt.Sprintf("%-31s Count: %d\n", "Total", len(listViolations)) + "```"
			return &discordgo.MessageEmbed{
				Title:       "Violations Summary",
				Description: out,
			}, nil
		},
	}

	cmdListV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ListViolations",
		Description:   "Lists Violations of specified user \n old flag posts oldest violations in first page ( from oldest to newest ).",
		Aliases:       []string{"Violations", "ViolationLogs", "VLogs", "VLog"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Page Number", Type: dcmd.Int, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "old", Name: "Oldest First"},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers, discordgo.PermissionKickMembers, discordgo.PermissionManageMessages},
		RunFunc: paginatedmessages.PaginatedCommand(1, func(parsed *dcmd.Data, p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			skip := (page - 1) * 15
			userID := parsed.Args[0].Int64()
			limit := 15

			//Check Flags
			order := "id desc"
			if parsed.Switches["old"].Value != nil && parsed.Switches["old"].Value.(bool) {
				order = "id asc"
			}

			// retrieve Violations
			listViolations, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ?", parsed.GS.ID, userID), qm.OrderBy(order), qm.Limit(limit), qm.Offset(skip)).AllG(context.Background())
			if err != nil {
				return nil, err
			}

			if len(listViolations) < 1 && p != nil && p.LastResponse != nil { //Dont send No Results error on first execution
				return nil, paginatedmessages.ErrNoResults
			}

			out := ""
			if len(listViolations) > 0 {
				for _, entry := range listViolations {

					out += fmt.Sprintf("#%-4d: [%-19s] Rule ID: %d \nViolation Name: %s\n\n", entry.ID, entry.CreatedAt.UTC().Format(time.RFC822), entry.RuleID.Int64, entry.Name)
				}

				out = "```" + out + "```"
			} else {
				out = "No violations"
			}

			return &discordgo.MessageEmbed{
				Title:       "Violation Logs",
				Description: out,
			}, nil
		}),
	}

	cmdDelV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "DeleteViolation",
		Description:   "Deletes a Violation with the specified ID. ID is the first number of each Violation in the ListViolations command.",
		Aliases:       []string{"DelViolation", "DelV", "DV"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "ID", Type: dcmd.Int},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			ID := parsed.Args[0].Int()
			rows, err := models.AutomodViolations(qm.Where("guild_id = ? AND id = ?", parsed.GS.ID, ID)).DeleteAll(context.Background(), common.PQ)

			if err != nil {
				return nil, err
			}
			if rows < 1 {
				return "Failed deleting, most likely no active violation with specified id", nil
			}

			return "👌", nil
		},
	}

	cmdClearV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ClearViolations",
		Description:   "Clears Violations of specified user optionally filtered by Name, Min/Max age and other conditions. By default, more recent violations are preferentially cleared.",
		Aliases:       []string{"ClearV", "ClrViolations", "ClrV"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Violation Name", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "ma", Name: "Max Violation Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Switch: "minage", Name: "Min Violation Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Switch: "num", Name: "Max Violations Cleared", Default: 0, Type: dcmd.Int},
			&dcmd.ArgDef{Switch: "old", Name: "Preferentially Clear Older Violations"},
			&dcmd.ArgDef{Switch: "skip", Name: "Amount Skipped", Default: 0, Type: dcmd.Int},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		GuildScopeCooldown:  2,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			UserID := parsed.Args[0].Int64()
			VName := parsed.Args[1].Str()
			order := "id desc"

			//Check Flags
			maxAge := parsed.Switches["ma"].Value.(time.Duration)
			minAge := parsed.Switches["minage"].Value.(time.Duration)
			skip := parsed.Switches["skip"].Int()
			if skip < 0 {
				skip = 0
			}
			limit := parsed.Switches["num"].Int()
			if parsed.Switches["old"].Value != nil && parsed.Switches["old"].Value.(bool) {
				order = "id asc"
			}

			//Construct Query and Fetch Rows
			qms := []qm.QueryMod{qm.Where("guild_id = ? AND user_id = ?", parsed.GS.ID, UserID), qm.OrderBy(order), qm.Offset(skip)}

			if VName != "" {
				qms = append(qms, qm.Where("name = ?", VName))
			}

			if maxAge != 0 {
				qms = append(qms, qm.Where("created_at > ?", time.Now().Add(-maxAge)))
			}

			if minAge != 0 {
				qms = append(qms, qm.Where("created_at < ?", time.Now().Add(-minAge)))
			}

			if limit > 0 {
				qms = append(qms, qm.Limit(limit))
			}

			rows, err := models.AutomodViolations(qms...).AllG(context.Background())
			if err != nil {
				return nil, err
			}

			//Delete Filtered rows.
			cleared, err := rows.DeleteAllG(context.Background())
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("%d Violations Cleared!!", cleared), nil
		},
	}

	container := commands.CommandSystem.Root.Sub("automod", "amod")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")

	container.AddCommand(cmdViewRulesets, cmdViewRulesets.GetTrigger())
	container.AddCommand(cmdToggleRuleset, cmdToggleRuleset.GetTrigger())
	container.AddCommand(cmdLogs, cmdLogs.GetTrigger())
	container.AddCommand(cmdListV, cmdListV.GetTrigger())
	container.AddCommand(cmdListVLC, cmdListVLC.GetTrigger())
	container.AddCommand(cmdDelV, cmdDelV.GetTrigger())
	container.AddCommand(cmdClearV, cmdClearV.GetTrigger())
}
