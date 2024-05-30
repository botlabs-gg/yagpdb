package topgames

import (
	"fmt"
	"sort"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

var Command = &commands.YAGCommand{
	Cooldown:     5,
	CmdCategory:  commands.CategoryDebug,
	Name:         "topgames",
	Description:  "Shows the top games on this server",
	HideFromHelp: true,
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "all"},
	},
	RunFunc: cmdFuncTopCommands,
}

func cmdFuncTopCommands(data *dcmd.Data) (interface{}, error) {
	allSwitch := data.Switch("all")
	var all bool
	if allSwitch != nil && allSwitch.Bool() {
		all = true

		if admin, err := bot.IsBotAdmin(data.Author.ID); !admin || err != nil {
			if err != nil {
				return nil, err
			}

			return "Only bot admins can check top games of all servers", nil
		}
	}

	// do it in 2 passes for speedy accumulation of data
	fastResult := make(map[string]int)

	if all {
		processShards := bot.ReadyTracker.GetProcessShards()
		for _, shard := range processShards {
			guilds := bot.State.GetShardGuilds(int64(shard))
			for _, g := range guilds {
				checkGuild(fastResult, g)
			}
		}
	} else {
		checkGuild(fastResult, data.GuildData.GS)
	}

	// then we convert and sort it
	fullResult := make([]*TopGameResult, 0, len(fastResult))
	for k, v := range fastResult {
		fullResult = append(fullResult, &TopGameResult{
			Game:  k,
			Count: v,
		})
	}

	sort.Slice(fullResult, func(i, j int) bool {
		return fullResult[i].Count > fullResult[j].Count
	})

	// display it
	out := ""
	if len(fullResult) > 0 {
		out = "```\nTop games being played currently\n#    Count -  Game\n"
	} else {
		out = "```\nNo Games being played currently"
	}

	for k, result := range fullResult {
		out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Game)
		if k >= 20 {
			break
		}
	}
	out += "\n```"

	return out, nil
}

func checkGuild(dst map[string]int, gs *dstate.GuildSet) {

	bot.State.IterateMembers(gs.ID, func(chunk []*dstate.MemberState) bool {
		for _, ms := range chunk {
			if ms.Presence == nil || ms.Presence.Game == nil || ms.Presence.Game.Name == "" {
				continue
			}

			if ms.User.Bot {
				continue
			}

			name := ms.Presence.Game.Name
			dst[name]++
		}

		return true
	})
}

type TopGameResult struct {
	Game  string
	Count int
}
