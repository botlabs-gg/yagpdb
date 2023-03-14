package currentshard

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/botrest"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "CurrentShard",
	Aliases:     []string{"cshard"},
	Description: "Shows the current shard this server is on (or the one specified)",
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "serverid", Type: dcmd.BigInt, Default: int64(0)},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		gID := data.GuildData.GS.ID

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
				status = "Unknown node... May not be running"
			} else {
				nodeStatus, err := botrest.GetNodeStatus(node.NodeID)
				if err != nil {
					status = "Failed querying status"
				} else {
					for _, v := range nodeStatus.Shards {
						if v.ShardID == shard {
							status = v.ConnStatus.String()
						}
					}
				}
			}
		}

		if status == "" {
			status = "Unknown"
		}

		return fmt.Sprintf("`%d` on shard `%d` out of total `%d` shards, status: `%s`", gID, shard, totalShards, status), nil
	},
}
