package currentshard

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "CurentShard",
	Aliases:              []string{"cshard"},
	Description:          "Shows the current shard this server is on",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		shard := bot.ShardManager.SessionForGuild(data.GS.ID())
		return fmt.Sprintf("On shard %d out of total %d shards.", shard.ShardID+1, shard.ShardCount), nil
	},
}
