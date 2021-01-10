package topgames

import (
	"fmt"
	"sort"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate/v2"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
)

var Command = &commands.YAGCommand{
	Cooldown:     5,
	CmdCategory:  commands.CategoryDebug,
	Name:         "topgames",
	Description:  "Shows the top games on this server",
	HideFromHelp: true,
	ArgSwitches: []*dcmd.ArgDef{
		&dcmd.ArgDef{Switch: "all"},
	},
	RunFunc: cmdFuncTopCommands,
}

func cmdFuncTopCommands(data *dcmd.Data) (interface{}, error) {
	allSwitch := data.Switch("all")
	var all bool
	if allSwitch != nil && allSwitch.Bool() {
		all = true

		if admin, err := bot.IsBotAdmin(data.Msg.Author.ID); !admin || err != nil {
			if err != nil {
				return nil, err
			}

			return "Only bot admins can check top games of all servers", nil
		}
	}

	// do it in 2 passes for speedy accumulation of data
	fastResult := make(map[string]int)

	if all {
		guilds := bot.State.GuildsSlice(true)
		for _, g := range guilds {
			checkGuild(fastResult, g)
		}
	} else {
		checkGuild(fastResult, data.GS)
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
	out := "```\nTop games being played currently\n#    Count -  Game\n"
	for k, result := range fullResult {
		out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Game)
		if k >= 20 {
			break
		}
	}
	out += "\n```"

	return out, nil
}

func checkGuild(dst map[string]int, gs *dstate.GuildState) {
	gs.RLock()
	defer gs.RUnlock()

	for _, ms := range gs.Members {
		if !ms.PresenceSet || ms.PresenceGame == nil || ms.PresenceGame.Name == "" {
			continue
		}

		if ms.Bot {
			continue
		}

		name := ms.PresenceGame.Name
		dst[name]++
	}
}

type TopGameResult struct {
	Game  string
	Count int
}
