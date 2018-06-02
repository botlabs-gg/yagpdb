package ping

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
	"time"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryTool,
	Name:        "Ping",
	Description: "I prefer tabletennis (Shows the bots ping to the discord servers)",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		return fmt.Sprintf(":PONG;%d", time.Now().UnixNano()), nil
	},
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()

	bUser := bot.State.User(true)
	if bUser == nil {
		return
	}

	if bUser.ID != m.Author.ID {
		return
	}

	// ping pong
	split := strings.Split(m.Content, ";")
	if split[0] != ":PONG" || len(split) < 2 {
		return
	}

	parsed, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return
	}

	taken := time.Duration(time.Now().UnixNano() - parsed)

	started := time.Now()
	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "Gateway (http send -> gateway receive time): "+taken.String())
	httpPing := time.Since(started)

	common.BotSession.ChannelMessageEdit(m.ChannelID, m.ID, "HTTP API (Edit Msg): "+httpPing.String()+"\nGateway: "+taken.String())
}
