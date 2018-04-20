package topservers

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"sort"
)

var yagCommand = commands.YAGCommand{
	Cooldown:    5,
	CmdCategory: commands.CategoryFun,
	Name:        "TopServers",
	Description: "Responds with the top 15 servers I'm on",

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		state := bot.State
		state.RLock()

		guilds := make([]*discordgo.Guild, len(state.Guilds))
		i := 0
		for _, v := range state.Guilds {
			state.RUnlock()
			guilds[i] = v.LightCopy(true)
			state.RLock()
			i++
		}
		state.RUnlock()

		sortable := GuildsSortUsers(guilds)
		sort.Sort(sortable)

		out := "```"
		for k, v := range sortable {
			if k > 14 {
				break
			}

			out += fmt.Sprintf("\n#%-2d: %-25s (%d members)", k+1, v.Name, v.MemberCount)
		}
		return "Top servers the bot is on (by membercount):\n" + out + "\n```", nil
	},
}

func Cmd() util.Command {
	return &cmd{}
}

type cmd struct {
	util.BaseCmd
}

func (c cmd) YAGCommand() *commands.YAGCommand {
	return &yagCommand
}

type GuildsSortUsers []*discordgo.Guild

func (g GuildsSortUsers) Len() int {
	return len(g)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (g GuildsSortUsers) Less(i, j int) bool {
	return g[i].MemberCount > g[j].MemberCount
}

// Swap swaps the elements with indexes i and j.
func (g GuildsSortUsers) Swap(i, j int) {
	temp := g[i]
	g[i] = g[j]
	g[j] = temp
}
