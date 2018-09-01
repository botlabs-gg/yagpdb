package findserver

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"strings"
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
	Description:          "Looks for a server by server name or the servers a user was on",
	HideFromHelp:         true,
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "name", Name: "name", Type: dcmd.String, Default: ""},
		&dcmd.ArgDef{Switch: "user", Name: "user", Type: dcmd.UserID, Default: 0},
	},
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		nameToMatch := strings.ToLower(data.Switch("name").Str())
		userIDToMatch := data.Switch("user").Int64()

		if userIDToMatch == 0 && nameToMatch == "" {
			return "-name or -user not provided", nil
		}

		candidates := make([]*Candidate, 0, 0)

		bot.State.RLock()
		for _, v := range bot.State.Guilds {
			bot.State.RUnlock()

			v.RLock()
			if candidate := CheckGuild(v, nameToMatch, userIDToMatch); candidate != nil {
				candidates = append(candidates, candidate)
			}
			v.RUnlock()

			bot.State.RLock()

			if len(candidates) > 1000 {
				break
			}
		}
		bot.State.RUnlock()

		if len(candidates) < 1 {
			return "No matches", nil
		}

		resp := ""
		for _, candidate := range candidates {
			resp += fmt.Sprintf("`%d`: **%s**", candidate.ID, candidate.Name)
			if candidate.UserMatch {
				resp += fmt.Sprintf(" (owner: `%t`, admin: `%t`, mod: `%t`)", candidate.Owner, candidate.Admin, candidate.Mod)
			}

			resp += "\n"
		}

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
