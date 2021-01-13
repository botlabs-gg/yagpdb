package memstats

import (
	"bytes"
	"encoding/json"
	"runtime"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/stdcommands/util"
)

var Command = &commands.YAGCommand{
	Cooldown:             2,
	CmdCategory:          commands.CategoryDebug,
	HideFromCommandsPage: true,
	Name:                 "memstats",
	Description:          ";))",
	HideFromHelp:         true,
	RunFunc: util.RequireOwner(func(data *dcmd.Data) (interface{}, error) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		buf, _ := json.Marshal(m)

		send := &discordgo.MessageSend{
			Content: "Memory stats",
			File: &discordgo.File{
				ContentType: "application/json",
				Name:        "memory_stats.json",
				Reader:      bytes.NewReader(buf),
			},
		}

		_, err := common.BotSession.ChannelMessageSendComplex(data.Msg.ChannelID, send)

		return nil, err
	}),
}
