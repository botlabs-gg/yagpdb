package commands

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"log"
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

				log.Println(within.String())

				var results []*TopCommandsResult
				err := common.SQL.Table(LoggedExecutedCommand{}.TableName()).Select("command, COUNT(id)").Where("created_at > ?", within).Group("command").Order("count(id) desc").Scan(&results).Error
				if err != nil {
					return "Uh oh something bad happened", err
				}

				out := "```"
				total := 0
				for k, result := range results {
					out += fmt.Sprintf("#%02d: %5d - %s\n", k+1, result.Count, result.Command)
					total += result.Count
				}

				cpm := float64(total) / float64(hours) / 60

				out += fmt.Sprintf("\nTotal: %d, Commands per minute: %0.1f", total, cpm)
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
