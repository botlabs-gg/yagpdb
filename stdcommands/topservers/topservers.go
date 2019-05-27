package topservers

import (
	"fmt"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot/models"
	"github.com/jonas747/yagpdb/commands"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var Command = &commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "TopServers",
	Description: "Responds with the top 15 servers I'm on",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Skip", Help: "Entries to skip", Type: dcmd.Int, Default: 0},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		skip := data.Args[0].Int()

		results, err := models.JoinedGuilds(qm.OrderBy("member_count desc"), qm.Limit(20), qm.Offset(skip)).AllG(data.Context())
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
