package moderation

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs"
	"time"
)

var (
	ErrFailedPerms = errors.New("Failed retrieving perms")
)

type ContextKey int

const (
	ContextKeyConfig ContextKey = iota
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(ModerationCommands...)
	bot.AddHandler(HandleGuildBanRemove, bot.EventGuildBanRemove)
	bot.AddHandler(bot.RedisWrapper(HandleMemberJoin), bot.EventGuildMemberAdd)
}

func HandleGuildBanRemove(ctx context.Context, evt interface{}) {
	r := evt.(*discordgo.GuildBanRemove)
	config, err := GetConfig(r.GuildID)
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving config")
		return
	}

	if !config.LogUnbans || config.ActionChannel == "" {
		return
	}

	embed := CreateModlogEmbed(nil, "Unbanned", r.User, "", "")
	_, err = common.BotSession.ChannelMessageSendEmbed(config.ActionChannel, embed)
	if err != nil {
		logrus.WithError(err).Error("Failed sending unban log message")
	}
}

func HandleMemberJoin(ctx context.Context, evt interface{}) {
	c := evt.(*discordgo.GuildMemberAdd)
	client := bot.ContextRedis(ctx)

	muteLeft, _ := client.Cmd("TTL", RedisKeyMutedUser(c.User.ID)).Int()
	if muteLeft < 10 {
		return
	}

	config, err := GetConfig(c.GuildID)
	if err != nil {
		logrus.WithError(err).WithField("guild", c.GuildID).Error("Failed retrieving config")
		return
	}
	if config.MuteRole == "" {
		return
	}

	logrus.WithField("guild", c.GuildID).WithField("user", c.User.ID).Info("Assigning back mute role after member rejoined")
	err = common.BotSession.GuildMemberRoleAdd(c.GuildID, c.User.ID, config.MuteRole)
	if err != nil {
		logrus.WithField("guild", c.GuildID).WithError(err).Error("Failed assigning mute role")
	}
}

const (
	ModCmdBan int = iota
	ModCmdKick
	ModCmdMute
	ModCmdUnMute
	ModCmdClean
	ModCmdReport
)

// ModBaseCmd is the base command for moderation commands, it makes sure proper permissions are there for the user invoking it
// and that the command is required and the reason is specified if required
func ModBaseCmd(neededPerm, cmd int, inner commandsystem.RunFunc) commandsystem.RunFunc {
	// userID, channelID, guildID string (config *Config, hasPerms bool, err error) {

	return func(data *commandsystem.ExecData) (interface{}, error) {

		userID := data.Message.Author.ID
		channelID := data.Channel.ID()
		guildID := data.Guild.ID()

		cmdName := data.Command.(*commandsystem.Command).Name

		if neededPerm != 0 {
			hasPerms, err := bot.AdminOrPerm(neededPerm, userID, channelID)
			if err != nil || !hasPerms {
				return fmt.Sprintf("The **%s** command requires the **%s** permission in this channel, you don't have it. (if you do contact bot support)", cmdName, common.StringPerms[neededPerm]), nil
			}
		}

		config, err := GetConfig(guildID)
		if err != nil {
			return "Error retrieving config", err
		}

		enabled := false
		reasonOptional := false
		reason := ""

		switch cmd {
		case ModCmdBan:
			enabled = config.BanEnabled
			reasonOptional = config.BanReasonOptional
			reason = data.SafeArgString(1)
		case ModCmdKick:
			enabled = config.KickEnabled
			reasonOptional = config.KickReasonOptional
			reason = data.SafeArgString(1)
		case ModCmdMute, ModCmdUnMute:
			enabled = config.MuteEnabled
			if cmd == ModCmdMute {
				reasonOptional = config.MuteReasonOptional
				reason = data.SafeArgString(2)
			} else {
				reasonOptional = config.UnmuteReasonOptional
				reason = data.SafeArgString(1)
			}
		case ModCmdClean:
			reasonOptional = true
			enabled = config.CleanEnabled
		case ModCmdReport:
			reasonOptional = true
			enabled = config.ReportEnabled
		default:
			panic("Unknown command")
		}

		if !enabled {
			return fmt.Sprintf("The **%s** command is disabled on this server. Enable it in the control panel", cmdName), nil
		}

		if !reasonOptional && reason == "" {
			return "Reason is required.", nil
		}

		return inner(data.WithContext(context.WithValue(data.Context(), ContextKeyConfig, config)))

	}
}

var ModerationCommands = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Command: &commandsystem.Command{
			Name:         "Ban",
			Description:  "Bans a member",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Reason", Type: commandsystem.ArgumentString},
			},
			Run: ModBaseCmd(discordgo.PermissionBanMembers, ModCmdBan, func(parsed *commandsystem.ExecData) (interface{}, error) {
				config := parsed.Context().Value(ContextKeyConfig).(*Config)

				reason := parsed.SafeArgString(1)
				if reason == "" {
					reason = "(No reason specified)"
				}
				target := parsed.Args[0].DiscordUser()

				err := BanUser(config, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, target)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			}),
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		Command: &commandsystem.Command{
			Name:         "Kick",
			Description:  "Kicks a member",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Reason", Type: commandsystem.ArgumentString},
			},
			Run: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdKick, func(parsed *commandsystem.ExecData) (interface{}, error) {
				config := parsed.Context().Value(ContextKeyConfig).(*Config)

				reason := parsed.SafeArgString(1)
				if reason == "" {
					reason = "(No reason specified)"
				}

				target := parsed.Args[0].DiscordUser()

				err := KickUser(config, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, target)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			}),
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		Command: &commandsystem.Command{
			Name:        "Mute",
			Description: "Mutes a member",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Minutes", Type: commandsystem.ArgumentNumber},
				&commandsystem.ArgDef{Name: "Reason", Type: commandsystem.ArgumentString},
			},
			ArgumentCombos: [][]int{[]int{0, 1, 2}, []int{0, 1}, []int{0, 2}, []int{0}},
			Run: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdMute, func(parsed *commandsystem.ExecData) (interface{}, error) {
				config := parsed.Context().Value(ContextKeyConfig).(*Config)

				reason := "(No reason specified)"
				if parsed.Args[2] != nil && parsed.Args[2].Str() != "" {
					reason = parsed.Args[2].Str()
				}

				muteDuration := 10
				if parsed.Args[1] != nil {
					muteDuration = parsed.Args[1].Int()
					if muteDuration < 1 || muteDuration > 1440 {
						return "Duration out of bounds (min 1, max 1440 - 1 day)", nil
					}
				}

				target := parsed.Args[0].DiscordUser()
				member := parsed.Guild.MemberCopy(true, target.ID, true)

				err := MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), true, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, member, muteDuration)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return "API Error: " + cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			}),
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		Command: &commandsystem.Command{
			Name:         "Unmute",
			Description:  "unmutes a member",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Reason", Type: commandsystem.ArgumentString},
			},
			Run: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdUnMute, func(parsed *commandsystem.ExecData) (interface{}, error) {
				config := parsed.Context().Value(ContextKeyConfig).(*Config)

				reason := "(No reason specified)"
				if parsed.Args[1] != nil && parsed.Args[1].Str() != "" {
					reason = parsed.Args[1].Str()
				} else if !config.UnmuteReasonOptional {
					return "No reason specified", nil
				}

				target := parsed.Args[0].DiscordUser()
				member := parsed.Guild.MemberCopy(true, target.ID, true)

				err := MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), false, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, member, 0)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return "API Error: " + cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			}),
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Cooldown:      5,
		Category:      commands.CategoryModeration,
		Command: &commandsystem.Command{
			Name:         "Report",
			Description:  "Reports a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "User", Type: commandsystem.ArgumentUser},
				&commandsystem.ArgDef{Name: "Reason", Type: commandsystem.ArgumentString},
			},
			Run: ModBaseCmd(0, ModCmdReport, func(parsed *commandsystem.ExecData) (interface{}, error) {
				config := parsed.Context().Value(ContextKeyConfig).(*Config)

				logLink := ""

				logs, err := logs.CreateChannelLog(parsed.Message.ChannelID, parsed.Message.Author.Username, parsed.Message.Author.ID, 100)
				if err != nil {
					logLink = "Log Creation failed"
					logrus.WithError(err).Error("Log Creation failed")
				} else {
					logLink = logs.Link()
				}

				channelID := config.ReportChannel
				if channelID == "" {
					channelID = parsed.Guild.ID()
				}

				reportBody := fmt.Sprintf("<@%s> Reported <@%s> in <#%s> For `%s`\nLast 100 messages from channel: <%s>", parsed.Message.Author.ID, parsed.Args[0].DiscordUser().ID, parsed.Message.ChannelID, parsed.Args[1].Str(), logLink)

				_, err = common.BotSession.ChannelMessageSend(channelID, reportBody)
				if err != nil {
					return "Failed sending report", err
				}

				// don't bother sending confirmation if it's in the same channel
				if channelID != parsed.Message.ChannelID {
					return "User reported to the proper authorities", nil
				}
				return "", nil
			}),
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Cooldown:      5,
		Category:      commands.CategoryModeration,
		Command: &commandsystem.Command{
			Name:                  "Clean",
			Description:           "Cleans the chat",
			Aliases:               []string{"clear", "cl"},
			RequiredArgs:          1,
			UserArgRequireMention: true,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Num", Type: commandsystem.ArgumentNumber},
				&commandsystem.ArgDef{Name: "User", Description: "Optionally specify a user, Deletions may be less than `num` if set", Type: commandsystem.ArgumentUser},
			},
			ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
			Run: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdClean, func(parsed *commandsystem.ExecData) (interface{}, error) {
				filter := ""
				if parsed.Args[1] != nil {
					filter = parsed.Args[1].DiscordUser().ID
				}

				num := parsed.Args[0].Int()
				if num > 100 {
					num = 100
				}

				if num < 1 {
					if num < 0 {
						return errors.New("Bot is having a stroke <https://www.youtube.com/watch?v=dQw4w9WgXcQ>"), nil
					}
					return errors.New("Can't delete nothing"), nil
				}

				limitFetch := num
				if filter != "" {
					limitFetch = num * 50 // Maybe just change to full fetch?
				}

				if limitFetch > 1000 {
					limitFetch = 1000
				}

				// Wait a second so the client dosen't gltich out
				time.Sleep(time.Second)

				numDeleted, err := DeleteMessages(parsed.Message.ChannelID, filter, num, limitFetch)

				return commandsystem.NewTemporaryResponse(time.Second*5, fmt.Sprintf("Deleted %d message(s)! :')", numDeleted)), err
			}),
		},
	},
}
