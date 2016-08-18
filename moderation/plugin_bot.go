package moderation

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

var (
	ErrFailedPerms = errors.New("Failed retrieving perms")
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(ModerationCommands...)
}

func AdminOrPerm(needed int, userID, channelID string) (bool, error) {
	perms, err := common.BotSession.State.UserChannelPermissions(userID, channelID)
	if err != nil {
		return false, err
	}

	if perms&needed != 0 {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 {
		return true, nil
	}

	return false, nil
}

var ModerationCommands = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Key: "moderation_ban_enabled:",
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

			ok, err := AdminOrPerm(discordgo.PermissionBanMembers, m.Author.ID, m.ChannelID)
			if err != nil {
				return ErrFailedPerms, err
			}
			if !ok {
				return "You have no admin or ban permissions >:(", nil
			}

			channelID, err := client.Cmd("GET", "moderation_action_channel:"+parsed.Guild.ID).Str()
			if err != nil {
				channelID = m.ChannelID
			}

			target := parsed.Args[0].DiscordUser()

			err = common.BotSession.GuildBanCreate(parsed.Guild.ID, target.ID, 1)
			if err != nil {
				return "API Refused to ban... (Bot probably dosen't have enough permissions)", err
			}

			log.Println("Banned ", parsed.Args[0].DiscordUser().Username, "cause", parsed.Args[1].Str())
			_, err = common.BotSession.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> Banned **%s** *(%s)*\n**Reason:** %s", m.Author.ID, target.Username, target.ID, parsed.Args[1].Str()))
			if err != nil {
				return "Failed sending report log", err
			}
			return "", nil
		},
	},
	&commands.CustomCommand{
		Key: "moderation_kick_enabled:",
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

			ok, err := AdminOrPerm(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID)
			if err != nil {
				return ErrFailedPerms, err
			}
			if !ok {
				return "You have no admin or kick permissions >:(", nil
			}

			channelID, err := client.Cmd("GET", "moderation_action_channel:"+parsed.Guild.ID).Str()
			if err != nil {
				channelID = m.ChannelID
			}

			target := parsed.Args[0].DiscordUser()

			err = common.BotSession.GuildMemberDelete(parsed.Guild.ID, target.ID)
			if err != nil {
				return "API Refused to kick... :/ (Bot probably dosen't have enough permissions)", err
			}

			log.Println("Kicked ", parsed.Args[0].DiscordUser().Username, "cause", parsed.Args[1].Str())
			_, err = common.BotSession.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> Kicked **%s** *(%s)*\n**Reason:** %s", m.Author.ID, target.Username, target.ID, parsed.Args[1].Str()))
			if err != nil {
				return "Failed sending kick repor in action channel", err
			}
			return "", nil
		},
	},
	&commands.CustomCommand{
		Key:      "moderation_report_enabled:",
		Cooldown: 5,
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

			// Send typing event to indicate the bot is working
			common.BotSession.ChannelTyping(m.ChannelID)

			logLink, err := common.CreateHastebinLog(m.ChannelID)
			if err != nil {
				return "Failed pastebin upload", err
			}

			channelID, err := client.Cmd("GET", "moderation_report_channel:"+parsed.Guild.ID).Str()
			if err != nil || channelID == "" {
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
		Key:      "moderation_clean_enabled:",
		Cooldown: 5,
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
			ok, err := AdminOrPerm(discordgo.PermissionManageMessages, m.Author.ID, m.ChannelID)
			if err != nil {
				return ErrFailedPerms, err
			}
			if !ok {
				return "You have no admin or manage messages permissions >:(", nil
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
					if len(ids) >= num {
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
