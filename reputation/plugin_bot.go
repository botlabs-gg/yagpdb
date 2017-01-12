package reputation

import (
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
	"time"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(cmds...)
	bot.AddHandler(bot.RedisWrapper(handleMessageCreate), bot.EventMessageCreate)
}

func handleMessageCreate(ctx context.Context, e interface{}) {
	evt := e.(*discordgo.MessageCreate)
	client := bot.ContextRedis(ctx)

	lower := strings.ToLower(evt.Content)
	if strings.Index(lower, "thanks") != 0 {
		return
	}

	if len(evt.Mentions) < 1 {
		return
	}

	who := evt.Mentions[0]

	if who.ID == evt.Author.ID {
		return
	}

	cs := bot.State.Channel(true, evt.ChannelID)

	enabled, _ := client.Cmd("GET", "reputation_enabled:"+cs.ID()).Bool()
	if !enabled {
		return
	}

	cooldown, err := CheckCooldown(client, cs.Guild.ID(), evt.Author.ID)
	if err != nil {
		log.WithError(err).Error("Failed checking cooldown for reputation")
		return
	}

	if cooldown > 0 {
		return
	}

	newScore, err := ModifyRep(client, 1, evt.Author, who, cs.Guild.ID())
	if err != nil {
		log.WithError(err).Error("Failed giving rep")
		return
	}

	msg := fmt.Sprintf("Gave +1 rep to **%s** *(%d rep total)*", who.Username, newScore)
	common.BotSession.ChannelMessageSend(evt.ChannelID, msg)
}

func ModifyRep(client *redis.Client, amount int, sender, target *discordgo.User, guildID string) (int, error) {
	settings, err := GetFullSettings(client, guildID)
	if err != nil {
		return 0, err
	}

	// Increase score
	newScoref, err := client.Cmd("ZINCRBY", "reputation_users:"+guildID, amount, target.ID).Float64()
	if err != nil {
		return 0, err
	}

	newScore := int(newScoref)

	// Set cooldown
	err = client.Cmd("SET", "reputation_cd:"+guildID+":"+sender.ID, time.Now().Unix()).Err
	if err != nil {
		return 0, err
	}

	// We don't care if an error occurs here
	err = client.Cmd("EXPIRE", "reputation_cd:"+guildID+":"+sender.ID, settings.Cooldown).Err
	if err != nil {
		log.WithError(err).Error("EXPIRE error")
	}

	return newScore, nil
}

func CheckCooldown(client *redis.Client, guildID, userID string) (int, error) {
	// Check for cooldown
	ttl, err := client.Cmd("TTL", "reputation_cd:"+guildID+":"+userID).Int()
	if err != nil {
		return 0, err
	}
	return ttl, nil
}

var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Key:      "reputation_enabled:",
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:         "GiveRep",
			Aliases:      []string{"+", "+rep"},
			Description:  "Gives +1 rep to someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				target := parsed.Args[0].DiscordUser()

				if target.ID == parsed.Message.Author.ID {
					return "Can't give rep to yourself... **silly**", nil
				}

				client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				timeLeft, err := CheckCooldown(client, parsed.Guild.ID(), parsed.Message.Author.ID)
				if err != nil {
					return "Failed checking cooldown", err
				}

				if timeLeft > 0 {
					return fmt.Sprintf("Still %d seconds left on cooldown", timeLeft), nil
				}

				newScore, err := ModifyRep(client, 1, parsed.Message.Author, target, parsed.Guild.ID())
				if err != nil {
					return "Failed giving rep >:I", err
				}

				msg := fmt.Sprintf("Gave +1 rep to **%s** *(%d rep total)*", target.Username, newScore)
				return msg, nil
			},
		},
	},
	&commands.CustomCommand{
		Key:      "reputation_enabled:",
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:         "RemoveRep",
			Aliases:      []string{"-", "-rep"},
			Description:  "Takes away 1 rep from someone",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				target := parsed.Args[0].DiscordUser()

				if target.ID == parsed.Message.Author.ID {
					return "Can't take away your own rep... **stopid**", nil
				}

				client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				timeLeft, err := CheckCooldown(client, parsed.Guild.ID(), parsed.Message.Author.ID)
				if err != nil {
					return "Failed checking cooldown", err
				}

				if timeLeft > 0 {
					return fmt.Sprintf("Still %d seconds left on cooldown", timeLeft), nil
				}

				newScore, err := ModifyRep(client, -1, parsed.Message.Author, target, parsed.Guild.ID())
				if err != nil {
					return "Failed removing rep >:I", err
				}

				msg := fmt.Sprintf("Removed 1 rep from **%s** *(%d rep total)*", target.Username, newScore)
				return msg, nil
			},
		},
	},
	&commands.CustomCommand{
		Key:      "reputation_enabled:",
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Rep",
			Description: "Shows yours or the specified users current rep and rank",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
			},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {
				target := parsed.Message.Author
				if parsed.Args[0] != nil {
					target = parsed.Args[0].DiscordUser()
				}

				client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
				score, rank, err := GetUserStats(client, parsed.Guild.ID(), target.ID)

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
	},
}
