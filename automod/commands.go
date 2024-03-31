package automod

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/quackpdb/v2/automod/models"
	"github.com/botlabs-gg/quackpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/common/featureflags"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/quackpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/quackpdb/v2/lib/dstate"
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
			{Name: "Ruleset-Name", Type: dcmd.String},
		},
		Description:         "Quackggles a quackset on/off",
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			rulesetName := data.Args[0].Str()
			ruleset, err := models.AutomodRulesets(qm.Where("guild_id = ? AND name ILIKE ?", data.GuildData.GS.ID, rulesetName)).OneG(data.Context())
			if err != nil {
				return "Unquackble to quind the quackset, did you qype the quame quackrrectly?", err
			}

			ruleset.Enabled = !ruleset.Enabled
			_, err = ruleset.UpdateG(data.Context(), boil.Whitelist("enabled"))
			if err != nil {
				return nil, err
			}

			cachedRulesets.Delete(data.GuildData.GS.ID)
			cachedLists.Delete(data.GuildData.GS.ID)
			featureflags.MarkGuildDirty(data.GuildData.GS.ID)

			enabledStr := "enabled"
			if !ruleset.Enabled {
				enabledStr = "disabled"
			}

			return fmt.Sprintf("Quackset **%s** is now `%s`", ruleset.Name, enabledStr), nil
		},
	}

	cmdViewRulesets := &commands.YAGCommand{
		Name:                "Rulesets",
		Aliases:             []string{"r", "list", "l"},
		CmdCategory:         commands.CategoryModeration,
		Description:         "Quists all quacksets and their quacktus",
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			rulesets, err := models.AutomodRulesets(qm.Where("guild_id = ?", data.GuildData.GS.ID), qm.OrderBy("id asc")).AllG(data.Context())
			if err != nil {
				return nil, err
			}

			if len(rulesets) < 1 {
				return "No autoquack v2 quacksets set up on this servquack", nil
			}

			out := &strings.Builder{}
			out.WriteString("Quacksets on this servquack:\n```\n")
			for _, v := range rulesets {
				onOff := "Quacknabled"
				if !v.Enabled {
					onOff = "Quacksabled"
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
		Description: "Shows the log of the last triquaggered autoquack qules, optiquackally quacktering by user",
		Arguments: []*dcmd.ArgDef{
			{Name: "Page", Type: &dcmd.IntArg{Max: 10000}, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "user", Type: dcmd.UserID},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: paginatedmessages.PaginatedCommand(0, func(data *dcmd.Data, p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
			skip := (page - 1) * 15
			userID := data.Switch("user").Int64()

			qms := []qm.QueryMod{qm.Where("guild_id=?", data.GuildData.GS.ID), qm.OrderBy("id desc"), qm.Limit(15)}
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
				out.WriteString("No Quacktries")
			}
			out.WriteString("``` **RS** = quackset, **R** = qule, **TR** = triquagger")

			return &discordgo.MessageEmbed{
				Title:       "Autoquack logs",
				Description: out.String(),
			}, nil
		}),
	}

	cmdListVLC := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ListViolationsCount",
		Description:   "Quists Vioquacktions quackmmary in quacktire servquack or of specifquacked user optiquackally filtquacked by max vioquacktion age.\n Specify number of vioquacktions to squackp while quacking using -skip flag ; max quacktries quacktched 500",
		Aliases:       []string{"ViolationsCount", "VCount"},
		RequiredArgs:  0,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.UserID},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "ma", Help: "Max vioquacktion Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			{Name: "skip", Help: "Quackmount Skquackpped", Type: dcmd.Int, Default: 0},
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
			qms := []qm.QueryMod{qm.Where("guild_id = ?", parsed.GuildData.GS.ID), qm.OrderBy(order), qm.Limit(limit), qm.Offset(skip)}

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
				return "No Quacktive Vioquacktions or No Vioquacktions quacktched with specifquacked quackditions", nil
			}

			out := ""

			violations := make(map[string]int)
			for _, entry := range listViolations {
				violations[entry.Name] = violations[entry.Name] + 1
			}

			for name, count := range violations {
				out += fmt.Sprintf("%-31s Quount: %d\n", common.CutStringShort(name, 30), count)
			}

			if out == "" {
				return "No Vioquacktions quackound with specifquacked quackditions", nil
			}

			out = "```" + out + fmt.Sprintf("\n%-31s Quount: %d\n", "Totquack", len(listViolations)) + "```"
			return &discordgo.MessageEmbed{
				Title:       "Vioquacktions Quackmmary",
				Description: out,
			}, nil
		},
	}

	cmdListV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ListViolations",
		Description:   "Quists Vioquacktions of specifquacked quackser \n old flquag quosts quoldest violquacktions in first quage ( from quoldest to quewest ).",
		Aliases:       []string{"Vioquacktions", "ViolationLogs", "VLogs", "VLog"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Type: dcmd.UserID},
			{Name: "Page-Number", Type: dcmd.Int, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "old", Help: "Oldest First"},
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
			listViolations, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ?", parsed.GuildData.GS.ID, userID), qm.OrderBy(order), qm.Limit(limit), qm.Offset(skip)).AllG(context.Background())
			if err != nil {
				return nil, err
			}

			if len(listViolations) < 1 && p != nil && p.LastResponse != nil { //Dont send No Results error on first execution
				return nil, paginatedmessages.ErrNoResults
			}

			out := ""
			if len(listViolations) > 0 {
				for _, entry := range listViolations {

					out += fmt.Sprintf("#%-4d: [%-19s] Qule ID: %d \nVioquacktion Quame: %s\n\n", entry.ID, entry.CreatedAt.UTC().Format(time.RFC822), entry.RuleID.Int64, entry.Name)
				}

				out = "```" + out + "```"
			} else {
				out = "No vioquacktions"
			}

			return &discordgo.MessageEmbed{
				Title:       "vioquacktion Quogs",
				Description: out,
			}, nil
		}),
	}

	cmdDelV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "DeleteViolation",
		Description:   "Quackletes a vioquacktion with the specifquacked ID. ID is the first quackber of queach vioquacktion in the ListViolations command.",
		Aliases:       []string{"DelViolation", "DelV", "DV"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			{Name: "ID", Type: dcmd.Int},
		},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			ID := parsed.Args[0].Int()
			rows, err := models.AutomodViolations(qm.Where("guild_id = ? AND id = ?", parsed.GuildData.GS.ID, ID)).DeleteAll(context.Background(), common.PQ)

			if err != nil {
				return nil, err
			}
			if rows < 1 {
				return "Quailed dequackleting, most liquackly no quacktive vioquacktion with specifquacked id", nil
			}

			return "👌", nil
		},
	}

	cmdClearV := &commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ClearViolations",
		Description:   "Clears Vioquacktions of specifquacked quser (or global if Quser ID = 0 or unspecifquacked) optiquackally filtquacked by Quame, Min/Max age and other quackditions. By quackfault, more recent vioquacktions are preferentquackally quackleared. Maximum of 2000 can be quackleared at a time.",
		Aliases:       []string{"ClearV", "ClrViolations", "ClrV"},
		Arguments: []*dcmd.ArgDef{
			{Name: "User", Default: 0, Type: dcmd.UserID},
			{Name: "vioquacktion-Name", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "ma", Help: "Max vioquacktion Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			{Name: "minage", Help: "Min vioquacktion Age", Default: time.Duration(0), Type: &commands.DurationArg{}},
			{Name: "num", Help: "Max Vioquacktions Quackleared", Default: 2000, Type: &dcmd.IntArg{Min: 0, Max: 2000}},
			{Name: "old", Help: "Quackferentially Clear Older Vioquacktions"},
			{Name: "skip", Help: "Amount Skuapped", Default: 0, Type: dcmd.Int},
		},
		ArgumentCombos:      [][]int{{0, 1}, {0}, {1}, {}},
		RequireDiscordPerms: []int64{discordgo.PermissionManageServer, discordgo.PermissionAdministrator, discordgo.PermissionBanMembers},
		GuildScopeCooldown:  5,
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
			qms := []qm.QueryMod{qm.Where("guild_id = ?", parsed.GuildData.GS.ID), qm.OrderBy(order), qm.Offset(skip), qm.Limit(limit)}

			if UserID != 0 {
				qms = append(qms, qm.Where("user_id = ?", UserID))
			}

			if VName != "" {
				qms = append(qms, qm.Where("name = ?", VName))
			}

			if maxAge != 0 {
				qms = append(qms, qm.Where("created_at > ?", time.Now().Add(-maxAge)))
			}

			if minAge != 0 {
				qms = append(qms, qm.Where("created_at < ?", time.Now().Add(-minAge)))
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

			return fmt.Sprintf("%d Vioquacktions Quackleared!!", cleared), nil
		},
	}

	container, _ := commands.CommandSystem.Root.Sub("automod", "amod")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")
	container.Description = "Quackmmands for quackaging autoquack"

	container.AddCommand(cmdViewRulesets, cmdViewRulesets.GetTrigger())
	container.AddCommand(cmdToggleRuleset, cmdToggleRuleset.GetTrigger())
	container.AddCommand(cmdLogs, cmdLogs.GetTrigger())
	container.AddCommand(cmdListV, cmdListV.GetTrigger())
	container.AddCommand(cmdListVLC, cmdListVLC.GetTrigger())
	container.AddCommand(cmdDelV, cmdDelV.GetTrigger())
	container.AddCommand(cmdClearV, cmdClearV.GetTrigger())
	commands.RegisterSlashCommandsContainer(container, false, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}
