package commands

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"sort"
	"time"
)

func requireOwner(inner commandsystem.RunFunc) commandsystem.RunFunc {
	return func(data *commandsystem.ExecData) (interface{}, error) {
		if data.Message.Author.ID != common.Conf.Owner {
			return "", nil
		}

		return inner(data)
	}
}

var debugCommands = []commandsystem.CommandHandler{
	&CustomCommand{
		Cooldown:             2,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		Command: &commandsystem.Command{
			Name:         "stateinfo",
			Description:  "Responds with state debug info",
			HideFromHelp: true,
			Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
				totalGuilds := 0
				totalMembers := 0
				totalChannels := 0
				totalMessages := 0

				state := bot.State
				state.RLock()
				defer state.RUnlock()

				totalGuilds = len(state.Guilds)

				for _, g := range state.Guilds {

					state.RUnlock()
					g.RLock()

					totalChannels += len(g.Channels)
					totalMembers += len(g.Members)

					for _, cState := range g.Channels {
						totalMessages += len(cState.Messages)
					}
					g.RUnlock()
					state.RLock()
				}

				embed := &discordgo.MessageEmbed{
					Title: "State size",
					Fields: []*discordgo.MessageEmbedField{
						&discordgo.MessageEmbedField{Name: "Guilds", Value: fmt.Sprint(totalGuilds), Inline: true},
						&discordgo.MessageEmbedField{Name: "Members", Value: fmt.Sprintf("%d", totalMembers), Inline: true},
						&discordgo.MessageEmbedField{Name: "Messages", Value: fmt.Sprintf("%d", totalMessages), Inline: true},
						&discordgo.MessageEmbedField{Name: "Channels", Value: fmt.Sprintf("%d", totalChannels), Inline: true},
					},
				}

				return embed, nil
			}),
		},
	}, &CustomCommand{
		Cooldown:             2,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		Command: &commandsystem.Command{
			Name:         "secretcommand",
			Description:  ";))",
			HideFromHelp: true,
			Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
				return "<@" + common.Conf.Owner + "> Is my owner", nil
			}),
		},
	},
	&CustomCommand{
		Cooldown:             2,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		Command: &commandsystem.Command{
			Name:         "topcommands",
			Description:  ";))",
			HideFromHelp: true,
			Arguments: []*commandsystem.ArgDef{
				{Name: "hours", Type: commandsystem.ArgumentNumber, Default: float64(1)},
			},
			Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {
				hours := data.Args[0].Int()
				within := time.Now().Add(time.Duration(-hours) * time.Hour)

				var results []*TopCommandsResult
				err := common.GORM.Table(LoggedExecutedCommand{}.TableName()).Select("command, COUNT(id)").Where("created_at > ?", within).Group("command").Order("count(id) desc").Scan(&results).Error
				if err != nil {
					return "Uh oh something bad happened", err
				}

				out := fmt.Sprintf("```\nCommand stats from now to %d hour(s) ago\n#    Total -  Command\n", hours)
				total := 0
				for k, result := range results {
					out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Command)
					total += result.Count
				}

				cpm := float64(total) / float64(hours) / 60

				out += fmt.Sprintf("\nTotal: %d, Commands per minute: %.1f", total, cpm)
				out += "\n```"

				return out, nil
			}),
		},
	},
	&CustomCommand{
		Cooldown:             2,
		Category:             CategoryFun,
		HideFromCommandsPage: true,
		Command: &commandsystem.Command{
			Name:         "topevents",
			Description:  ";))",
			HideFromHelp: true,
			Run: requireOwner(func(data *commandsystem.ExecData) (interface{}, error) {

				bot.EventLogger.Lock()

				sortable := make([]*DiscordEvtEntry, len(bot.EventLogger.Events))

				i := 0
				for k, v := range bot.EventLogger.Events {
					sortable[i] = &DiscordEvtEntry{
						Name:  k,
						Count: v,
					}
					i++
				}

				bot.EventLogger.Unlock()

				sort.Sort(DiscordEvtEntrySortable(sortable))

				uptime := time.Since(bot.Started)

				out := "```\n#   Total  -  Avg/m  - Event\n"
				total := 0
				for k, entry := range sortable {
					out += fmt.Sprintf("#%-2d: %5d - %5.2f - %s\n", k+1, entry.Count, float64(entry.Count)/(uptime.Seconds()/60), entry.Name)
					total += entry.Count
				}

				epm := float64(total) / (uptime.Seconds() / 60)

				out += fmt.Sprintf("\nTotal: %d, Events per minute: %.1f", total, epm)
				out += "\n```"

				return out, nil
			}),
		},
	},
}

type TopCommandsResult struct {
	Command string
	Count   int
}

type DiscordEvtEntry struct {
	Name  string
	Count int
}

type DiscordEvtEntrySortable []*DiscordEvtEntry

func (d DiscordEvtEntrySortable) Len() int {
	return len(d)
}

func (d DiscordEvtEntrySortable) Less(i, j int) bool {
	return d[i].Count > d[j].Count
}

func (d DiscordEvtEntrySortable) Swap(i, j int) {
	temp := d[i]
	d[i] = d[j]
	d[j] = temp
}
