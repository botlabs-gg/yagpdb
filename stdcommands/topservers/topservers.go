package topservers

import (
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"sort"
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

		entriesIncluded := 0
		out := "```"
		for k, v := range sortable {
			if entriesIncluded > 14 {
				break
			}

			if k < skip {
				continue
			}

			entriesIncluded++
			out += fmt.Sprintf("\n#%-2d: %-25s (%d members)", k+1, v.Name, v.MemberCount)
		}
		return "Top servers the bot is on (by membercount):\n" + out + "\n```", nil
	},
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
