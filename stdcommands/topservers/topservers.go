package topservers

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot/models"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "TopServers",
	Description: "Responds with the top 20 servers I'm on",
	Arguments: []*dcmd.ArgDef{
		{Name: "Skip", Help: "Entries to skip", Type: dcmd.Int, Default: 0},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "id", Type: dcmd.BigInt},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		skip := data.Args[0].Int()

		if data.Switches["id"].Value != nil {
			type serverIDQuery struct {
				MemberCount int64
				Name        string
				Place       int64
			}
			var serverID int64
			var position serverIDQuery
			serverID = data.Switch("id").Int64()
			const q = `SELECT member_count, name, row_number FROM (SELECT id, member_count, name, left_at, row_number() OVER (ORDER BY member_count DESC) FROM joined_guilds WHERE left_at IS NULL) AS total WHERE id=$1 AND left_at IS NULL;`
			err := common.PQ.QueryRow(q, serverID).Scan(&position.MemberCount, &position.Name, &position.Place)
			return fmt.Sprintf("```Server with ID %d is placed:\n#%-2d: %-25s (%d members)\n```", serverID, position.Place, position.Name, position.MemberCount), err
		}

		results, err := models.JoinedGuilds(qm.Where("left_at is null"), qm.OrderBy("member_count desc"), qm.Limit(20), qm.Offset(skip)).AllG(data.Context())
		if err != nil {
			return nil, err
		}

		out := "```"
		for k, v := range results {
			out += fmt.Sprintf("\n#%-2d: %-25s (%d members)", k+skip+1, v.Name, v.MemberCount)
		}
		return "Top servers the bot is on:\n" + out + "\n```", nil
	},
}
