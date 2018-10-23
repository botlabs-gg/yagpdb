package currentshard

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "CurrentShard",
	Aliases:     []string{"cshard"},
	Description: "Shows the current shard this server is on (or the one specified",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "serverid", Type: dcmd.Int, Default: int64(0)},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		gID := data.GS.ID

		if data.Args[0].Int64() != 0 {
			gID = data.Args[0].Int64()
		}

		shard := bot.ShardManager.SessionForGuild(gID)
		if shard == nil {
			return "Unknown shard...?", nil
		}

		status := shard.GatewayManager.Status()

		return fmt.Sprintf("`%d` on shard `%d` out of total `%d` shards, status: `%s`", gID, shard.ShardID, shard.ShardCount, status.String()), nil
	},
}
