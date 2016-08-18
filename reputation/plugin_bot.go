package reputation

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"log"
	"strconv"
	"time"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(cmds...)
}

var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Key: "reputation_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "GiveRep",
			Aliases:      []string{"+", "+rep"},
			Description:  "Gives +1 rep to someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			target := parsed.Args[0].DiscordUser()

			if target.ID == m.Author.ID {
				return "Can't give rep to yourself... **silly**", nil
			}

			channel := parsed.Channel

			// Check for cooldown
			lastUsed, err := client.Cmd("GET", "reputation_cd:"+channel.GuildID+":"+m.Author.ID).Int64()
			if err != nil {
				lastUsed = 0
			}

			settings, err := GetFullSettings(client, channel.GuildID)
			if err != nil {
				return "Error retrieving reputation settings", err
			}

			timeSinceLast := time.Since(time.Unix(lastUsed, 0))
			timeLeft := settings.Cooldown - int(timeSinceLast.Seconds())

			if timeLeft > 0 && lastUsed > 0 {
				return fmt.Sprintf("Still %d seconds left on cooldown", timeLeft), nil
			}

			// Increase score
			newScoref, err := client.Cmd("ZINCRBY", "reputation_users:"+channel.GuildID, 1, target.ID).Float64()
			if err != nil {
				log.Println("Failed setting new score", err)
				return "Failed setting new reputation score", err
			}

			newScore := int64(newScoref)

			// Set cooldown
			err = client.Cmd("SET", "reputation_cd:"+channel.GuildID+":"+m.Author.ID, time.Now().Unix()).Err
			if err != nil {
				return "Failed setting new cooldown", err
			}

			// We don't care if an error occurs here
			err = client.Cmd("EXPIRE", "reputation_cd:"+channel.GuildID+":"+m.Author.ID, settings.Cooldown).Err
			if err != nil {
				log.Println("EPIRE err", err)
			}

			msg := fmt.Sprintf("Gave +1 rep to **%s** *(%d rep total)*", target.Username, newScore)
			return msg, nil
		},
	},
	&commands.CustomCommand{
		Key: "reputation_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "rep",
			Description: "Shows yours or the specified users current rep and rank",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			target := m.Author
			if parsed.Args[0] != nil {
				target = parsed.Args[0].DiscordUser()
			}

			channel := parsed.Channel

			score, rank, err := GetUserStats(client, channel.GuildID, target.ID)

			if err != nil {
				if err == ErrUserNotFound {
					rank = -1
				} else {
					return "Error retrieving stats", err
				}
			}

			rankStr := "∞"
			if rank != -1 {
				rankStr = strconv.FormatInt(int64(rank)+1, 10)
			}

			return fmt.Sprintf("**%s**: **%d** Rep (#**%s**)", target.Username, score, rankStr), nil
		},
	},
}
