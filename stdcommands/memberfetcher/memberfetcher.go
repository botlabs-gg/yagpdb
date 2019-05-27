package memberfetcher

import (
	"fmt"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryDebug,
	Name:        "MemberFetcher",
	Aliases:     []string{"memfetch"},
	Description: "Shows the current status of the member fetcher",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		fetching, notFetching := bot.MemberFetcher.Status()
		return fmt.Sprintf("Fetching: `%d`, Not fetching: `%d`", fetching, notFetching), nil
	},
}
