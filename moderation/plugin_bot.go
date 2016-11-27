package moderation

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
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
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Ban",
			Description:  "Bans a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			config, perm, err := BaseCmd(discordgo.PermissionBanMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.BanEnabled {
				return "Ban command disabled.", nil
			}
			if !perm {
				return "You do not have ban permissions.", nil
			}

			target := parsed.Args[0].DiscordUser()

			err = BanUser(config, parsed.Guild.ID, m.ChannelID, "<@"+m.Author.ID+">", parsed.Args[1].Str(), target)
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
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Kick",
			Description:  "Kicks a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			config, perm, err := BaseCmd(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.KickEnabled {
				return "Kick command disabled.", nil
			}
			if !perm {
				return "You do not have kick permissions.", nil
			}

			target := parsed.Args[0].DiscordUser()

			err = KickUser(config, parsed.Guild.ID, m.ChannelID, "<@"+m.Author.ID+">", parsed.Args[1].Str(), target)
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
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Mute",
			Description:  "Mutes a member",
			RequiredArgs: 3,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Minutes", Type: commandsystem.ArgumentTypeNumber},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			config, perm, err := BaseCmd(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.MuteEnabled {
				return "Mute command disabled.", nil
			}
			if !perm {
				return "You do not have kick (Required for mute) permissions.", nil
			}

			muteDuration := parsed.Args[1].Int()
			if muteDuration < 1 || muteDuration > 1440 {
				return "Duration out of bounds (min 1, max 1440 - 1 day)", nil
			}

			target := parsed.Args[0].DiscordUser()

			member, err := common.BotSession.State.Member(parsed.Guild.ID, target.ID)
			if err != nil {
				return "I COULDNT FIND ZE GUILDMEMEBER PLS HELP AAAAAAA", err
			}

			err = MuteUnmuteUser(config, client, true, parsed.Guild.ID, m.ChannelID, "<@"+m.Author.ID+">", parsed.Args[2].Str(), member, parsed.Args[1].Int())
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
	&commands.CustomCommand{
		CustomEnabled: true,
		Category:      commands.CategoryModeration,
		Cooldown:      5,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Unmute",
			Description:  "unmutes a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			config, perm, err := BaseCmd(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.MuteEnabled {
				return "Mute command disabled.", nil
			}
			if !perm {
				return "You do not have kick (Required for mute) permissions.", nil
			}

			target := parsed.Args[0].DiscordUser()

			member, err := common.BotSession.State.Member(parsed.Guild.ID, target.ID)
			if err != nil {
				return "I COULDNT FIND ZE GUILDMEMEBER PLS HELP AAAAAAA", err
			}

			err = MuteUnmuteUser(config, client, false, parsed.Guild.ID, m.ChannelID, "<@"+m.Author.ID+">", parsed.Args[1].Str(), member, 0)
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
	&commands.CustomCommand{
		CustomEnabled: true,
		Cooldown:      5,
		Category:      commands.CategoryModeration,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Report",
			Description:  "Reports a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			config, _, err := BaseCmd(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.ReportEnabled {
				return "Mute command disabled.", nil
			}

			logLink := ""

			logs, err := logs.CreateChannelLog(m.ChannelID, m.Author.Username, m.Author.ID, 100)
			if err != nil {
				logLink = "Log Creation failed"
				logrus.WithError(err).Error("Log Creation failed")
			} else {
				logLink = logs.Link()
			}

			channelID := config.ReportChannel
			if channelID == "" {
				channelID = parsed.Guild.ID
			}

			reportBody := fmt.Sprintf("<@%s> Reported <@%s> For %s\nLast 100 messages from channel: <%s>", m.Author.ID, parsed.Args[0].DiscordUser().ID, parsed.Args[1].Str(), logLink)

			_, err = common.BotSession.ChannelMessageSend(channelID, reportBody)
			if err != nil {
				return "Failed sending report", err
			}

			// don't bother sending confirmation if it's in the same channel
			if channelID != m.ChannelID {
				return "User reported to the proper authorities", nil
			}
			return "", nil
		},
	},
	&commands.CustomCommand{
		CustomEnabled: true,
		Cooldown:      5,
		Category:      commands.CategoryModeration,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:                  "Clean",
			Description:           "Cleans the chat",
			RequiredArgs:          1,
			UserArgRequireMention: true,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Num", Type: commandsystem.ArgumentTypeNumber},
				&commandsystem.ArgumentDef{Name: "User", Description: "Optionally specify a user, Deletions may be less than `num` if set", Type: commandsystem.ArgumentTypeUser},
			},
			ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			config, perm, err := BaseCmd(discordgo.PermissionManageMessages, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !config.CleanEnabled {
				return "Clean command disabled.", nil
			}
			if !perm {
				return "You do not have manage messages permissions in this channel.", nil
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

			msgs, err := common.GetMessages(m.ChannelID, limitFetch)

			ids := make([]string, 0)
			for i := len(msgs) - 1; i >= 0; i-- {
				//log.Println(msgs[i].ID, msgs[i].ContentWithMentionsReplaced())
				if (filter == "" || msgs[i].Author.ID == filter) && msgs[i].ID != m.ID {
					ids = append(ids, msgs[i].ID)
					//log.Println("Deleting", msgs[i].ContentWithMentionsReplaced())
					if len(ids) >= num || len(ids) >= 99 {
						break
					}
				}
			}
			ids = append(ids, m.ID)

			if len(ids) < 2 {
				return "Deleted nothing... sorry :(", nil
			}

			var delMsg *discordgo.Message
			delMsg, err = common.BotSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Deleting %d messages! :')", len(ids)))
			// Self destruct in 3...
			if err == nil {
				go common.DelayedMessageDelete(common.BotSession, time.Second*5, delMsg.ChannelID, delMsg.ID)
			}

			// Wait a second so the client dosen't gltich out
			time.Sleep(time.Second)
			err = common.BotSession.ChannelMessagesBulkDelete(m.ChannelID, ids)

			return "", err
		},
	},
}
