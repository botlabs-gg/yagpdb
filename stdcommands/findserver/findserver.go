package findserver

import (
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/bot/models"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands/util"
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
		{Name: "name", Type: dcmd.String, Default: ""},
		{Name: "user", Type: dcmd.UserID, Default: 0},
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

func CheckGuild(gs *dstate.GuildSet, nameToMatch string, userToMatch int64) *Candidate {
	if nameToMatch != "" {
		gl := strings.ToLower(gs.Name)
		if gl != nameToMatch && !strings.Contains(gl, nameToMatch) {
			return nil
		}
	}

	candidate := &Candidate{
		ID:   gs.ID,
		Name: gs.Name,
	}

	return candidate
}
