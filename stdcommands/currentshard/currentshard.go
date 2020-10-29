package currentshard

import (
	"fmt"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
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

		totalShards := bot.ShardManager.GetNumShards()
		shard := bot.GuildShardID(int64(totalShards), gID)

		status := ""
		if bot.ReadyTracker.IsGuildOnProcess(gID) {
			session := bot.ShardManager.SessionForGuild(gID)
			if session == nil {
				return "Unknown shard...?", nil
			}

			status = session.GatewayManager.Status().String()
		} else {
			node, err := common.ServicePoller.GetShardNode(shard)
			if err != nil {
				status = "Uknown node... May not be running"
			}

			nodeStatus, err := botrest.GetNodeStatus(node.NodeID)
			if err != nil {
				status = "failed querying status"
			} else {
				for _, v := range nodeStatus.Shards {
					if v.ShardID == shard {
						status = v.ConnStatus.String()
					}
				}
			}

			status = "unknown (on another node than this one)"
		}

		return fmt.Sprintf("`%d` on shard `%d` out of total `%d` shards, status: `%s`", gID, shard, totalShards, status), nil
	},
}
