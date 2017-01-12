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

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(ModerationCommands...)
	bot.AddHandler(HandleGuildBanRemove, bot.EventGuildBanRemove)
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

func BaseCmd(neededPerm int, userID, channelID, guildID string) (config *Config, hasPerms bool, err error) {
	if neededPerm != 0 {
		hasPerms, err = common.AdminOrPerm(neededPerm, userID, channelID)
		if err != nil || !hasPerms {
			return
		}
	}

	config, err = GetConfig(guildID)
	return
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
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, perm, err := BaseCmd(discordgo.PermissionBanMembers, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !perm {
					return "You do not have ban permissions.", nil
				}
				if !config.BanEnabled {
					return "Ban command disabled.", nil
				}

				reason := "(No reason specified)"
				if parsed.Args[1] != nil && parsed.Args[1].Str() != "" {
					reason = parsed.Args[1].Str()
				} else if !config.BanReasonOptional {
					return "No reason specified", nil
				}

				target := parsed.Args[0].DiscordUser()

				err = BanUser(config, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, target)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			},
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
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, perm, err := BaseCmd(discordgo.PermissionKickMembers, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !perm {
					return "You do not have kick permissions.", nil
				}
				if !config.KickEnabled {
					return "Kick command disabled.", nil
				}

				reason := "(No reason specified)"
				if parsed.Args[1] != nil && parsed.Args[1].Str() != "" {
					reason = parsed.Args[1].Str()
				} else if !config.KickReasonOptional {
					return "No reason specified", nil
				}

				target := parsed.Args[0].DiscordUser()

				err = KickUser(config, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, target)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			},
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
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, perm, err := BaseCmd(discordgo.PermissionKickMembers, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !perm {
					return "You do not have kick (Required for mute) permissions.", nil
				}
				if !config.MuteEnabled {
					return "Mute command disabled.", nil
				}

				reason := "(No reason specified)"
				if parsed.Args[2] != nil && parsed.Args[2].Str() != "" {
					reason = parsed.Args[2].Str()
				} else if !config.MuteReasonOptional {
					return "No reason specified", nil
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

				err = MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), true, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, member, muteDuration)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return "API Error: " + cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			},
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
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, perm, err := BaseCmd(discordgo.PermissionKickMembers, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !perm {
					return "You do not have kick (Required for mute) permissions.", nil
				}
				if !config.MuteEnabled {
					return "Mute command disabled.", nil
				}

				reason := "(No reason specified)"
				if parsed.Args[1] != nil && parsed.Args[1].Str() != "" {
					reason = parsed.Args[1].Str()
				} else if !config.UnmuteReasonOptional {
					return "No reason specified", nil
				}

				target := parsed.Args[0].DiscordUser()
				member := parsed.Guild.MemberCopy(true, target.ID, true)

				err = MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), false, parsed.Guild.ID(), parsed.Message.ChannelID, parsed.Message.Author, reason, member, 0)
				if err != nil {
					if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
						return "API Error: " + cast.Message.Message, err
					} else {
						return "An error occurred", err
					}
				}

				return "", nil
			},
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
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, _, err := BaseCmd(0, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !config.ReportEnabled {
					return "Report command disabled.", nil
				}

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

				reportBody := fmt.Sprintf("<@%s> Reported <@%s> in <#%s> For `%s`\nLast 100 messages from channel: <%s>", parsed.Message.Author, parsed.Args[0].DiscordUser().ID, parsed.Message.ChannelID, parsed.Args[1].Str(), logLink)

				_, err = common.BotSession.ChannelMessageSend(channelID, reportBody)
				if err != nil {
					return "Failed sending report", err
				}

				// don't bother sending confirmation if it's in the same channel
				if channelID != parsed.Message.ChannelID {
					return "User reported to the proper authorities", nil
				}
				return "", nil
			},
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Cooldown:      5,
		Category:      commands.CategoryModeration,
		Command: &commandsystem.Command{
			Name:                  "Clean",
			Description:           "Cleans the chat",
			RequiredArgs:          1,
			UserArgRequireMention: true,
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Num", Type: commandsystem.ArgumentNumber},
				&commandsystem.ArgDef{Name: "User", Description: "Optionally specify a user, Deletions may be less than `num` if set", Type: commandsystem.ArgumentUser},
			},
			ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
			Run: func(parsed *commandsystem.ExecData) (interface{}, error) {

				config, perm, err := BaseCmd(discordgo.PermissionManageMessages, parsed.Message.Author.ID, parsed.Message.ChannelID, parsed.Guild.ID())
				if err != nil {
					return "Error retrieving config.", err
				}
				if !perm {
					return "You do not have manage messages permissions in this channel.", nil
				}
				if !config.CleanEnabled {
					return "Clean command disabled.", nil
				}

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

				msgs, err := bot.GetMessages(parsed.Message.ChannelID, limitFetch)

				ids := make([]string, 0)
				for i := len(msgs) - 1; i >= 0; i-- {
					//log.Println(msgs[i].ID, msgs[i].ContentWithMentionsReplaced())
					if (filter == "" || msgs[i].Author.ID == filter) && msgs[i].ID != parsed.Message.ID {
						ids = append(ids, msgs[i].ID)
						//log.Println("Deleting", msgs[i].ContentWithMentionsReplaced())
						if len(ids) >= num || len(ids) >= 99 {
							break
						}
					}
				}
				ids = append(ids, parsed.Message.ID)

				if len(ids) < 2 {
					return "Deleted nothing... sorry :(", nil
				}

				var delMsg *discordgo.Message
				delMsg, err = common.BotSession.ChannelMessageSend(parsed.Message.ChannelID, fmt.Sprintf("Deleting %d messages! :')", len(ids)))
				// Self destruct in 3...
				if err == nil {
					go common.DelayedMessageDelete(common.BotSession, time.Second*5, delMsg.ChannelID, delMsg.ID)
				}

				// Wait a second so the client dosen't gltich out
				time.Sleep(time.Second)
				err = common.BotSession.ChannelMessagesBulkDelete(parsed.Message.ChannelID, ids)

				return "", err
			},
		},
	},
}
