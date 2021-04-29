package findserver

import (
	"fmt"
	"strings"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v2"
	"github.com/jonas747/yagpdb/bot/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

type Candidate struct {
	ID   int64
	Name string

	UserMatch bool
	Owner     bool
	Admin     bool
	Mod       bool
}

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "findserver",
	Aliases:              []string{"findservers"},
	Description:          "Looks for a server by server name or the servers a user owns",
	HideFromHelp:         true,
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "name", Name: "name", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "user", Name: "user", Type: dcmd.UserID, Default: 0},
	},
	RunFunc: util.RequireBotAdmin(func(data *dcmd.Data) (interface{}, error) {
		nameToMatch := strings.ToLower(data.Switch("name").Str())
		userIDToMatch := data.Switch("user").Int64()

		if userIDToMatch == 0 && nameToMatch == "" {
			return "-name or -user not provided", nil
		}

		var whereQM qm.QueryMod
		if userIDToMatch != 0 {
			whereQM = qm.Where("owner_id = ?", userIDToMatch)
		} else {
			whereQM = qm.Where("name ILIKE ?", "%"+nameToMatch+"%")
		}

		results, err := models.JoinedGuilds(qm.Where("left_at is null"), whereQM, qm.OrderBy("id desc"), qm.Limit(250)).AllG(data.Context())
		if err != nil {
			return nil, err
		}

		resp := ""
		for _, v := range results {
			resp += fmt.Sprintf("`%d`: **%s**\n", v.ID, v.Name)
		}

		resp += fmt.Sprintf("%d results", len(results))

		return resp, nil
	}),
}

func CheckGuild(gs *dstate.GuildState, nameToMatch string, userToMatch int64) *Candidate {
	if nameToMatch != "" {
		gl := strings.ToLower(gs.Guild.Name)
		if gl != nameToMatch && !strings.Contains(gl, nameToMatch) {
			return nil
		}
	}

	foundUser := false
	if userToMatch != 0 {
		for _, ms := range gs.Members {
			if ms.ID == userToMatch {
				foundUser = true
				break
			}
		}

		if !foundUser {
			return nil
		}
	}

	candidate := &Candidate{
		ID:   gs.ID,
		Name: gs.Guild.Name,
	}

	if foundUser {
		if gs.Guild.OwnerID == userToMatch {
			candidate.Owner = true
		}

		perms, _ := gs.MemberPermissions(false, 0, userToMatch)
		if perms&discordgo.PermissionAdministrator != 0 {
			candidate.Admin = true
		}

		if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionKickMembers != 0 || perms&discordgo.PermissionBanMembers != 0 {
			candidate.Mod = true
		}

		candidate.UserMatch = true
	}

	return candidate
}
