package reputation

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"log"
	"strconv"
	"time"
)

func (p *Plugin) InitBot() {
	bot.CommandSystem.RegisterCommands(commands...)
}

var commands = []commandsystem.CommandHandler{
	&bot.CustomCommand{
		Key: "reputation_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "+",
			Aliases:      []string{"giverep", "+rep"},
			Description:  "Gives +1 rep to someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				target := parsed.Args[0].DiscordUser()

				if target.ID == m.Author.ID {
					return errors.New("Can't give rep to yourself... **silly**")
				}

				channel, err := bot.Session.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				// Check for cooldown
				lastUsed, err := client.Cmd("GET", "reputation_cd:"+channel.GuildID+":"+m.Author.ID).Int64()
				if err != nil {
					lastUsed = 0
				}

				settings, err := GetFullSettings(client, channel.GuildID)
				if err != nil {
					log.Println("Failed retrieving settings", err)
					return err
				}

				timeSinceLast := time.Since(time.Unix(lastUsed, 0))
				timeLeft := settings.Cooldown - int(timeSinceLast.Seconds())

				if timeLeft > 0 && lastUsed > 0 {
					return fmt.Errorf("Still %d seconds left on cooldown", timeLeft)
				}

				// Increase score
				newScoref, err := client.Cmd("ZINCRBY", "reputation_users:"+channel.GuildID, 1, target.ID).Float64()
				if err != nil {
					log.Println("Failed setting new score", err)
					return err
				}

				newScore := int64(newScoref)

				// Set cooldown
				err = client.Cmd("SET", "reputation_cd:"+channel.GuildID+":"+m.Author.ID, time.Now().Unix()).Err
				if err != nil {
					log.Println("Failed setting new cooldown", err)
					return err
				}

				// We don't care if an error occurs here
				err = client.Cmd("EXPIRE", "reputation_cd:"+channel.GuildID+":"+m.Author.ID, settings.Cooldown).Err
				if err != nil {
					log.Println("EPIRE err", err)
				}

				msg := fmt.Sprintf("Gave +1 rep to **%s** *(%d rep total)*", target.Username, newScore)
				common.BotSession.ChannelMessageSend(m.ChannelID, msg)
				return nil
			},
		},
	},
	&bot.CustomCommand{
		Key: "reputation_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "rep",
			Description: "Shows yours or the specified users current rep and rank",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				target := m.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
				}

				channel, err := bot.Session.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				score, rank, err := GetUserStats(client, channel.GuildID, target.ID)

				if err != nil {
					if err == ErrUserNotFound {
						rank = -1
					} else {
						return err
					}
				}

				rankStr := "∞"
				if rank != -1 {
					rankStr = strconv.FormatInt(int64(rank)+1, 10)
				}

				msg := fmt.Sprintf("**%s**: **%d** Rep (#**%s**)", target.Username, score, rankStr)
				common.BotSession.ChannelMessageSend(m.ChannelID, msg)
				return nil
			},
		},
	},
}
