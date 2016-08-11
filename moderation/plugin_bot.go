package moderation

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

func (p *Plugin) InitBot() {
	bot.CommandSystem.RegisterCommands(ModerationCommands...)
}

func AdminOrPerm(in int, perm int) bool {
	if in&perm != 0 {
		return true
	}

	if in&discordgo.PermissionManageServer != 0 {
		return true
	}

	return false
}

var ModerationCommands = []commandsystem.CommandHandler{
	&bot.CustomCommand{
		Key: "moderation_ban_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Ban",
			Description:  "Bans a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				perms, err := common.BotSession.State.UserChannelPermissions(m.Author.ID, m.ChannelID)
				if err != nil {
					return err
				}

				if !AdminOrPerm(perms, discordgo.PermissionBanMembers) {
					return errors.New("User has no admin or ban permissions")
				}

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				channel, err := common.BotSession.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				channelID, err := client.Cmd("GET", "moderation_action_channel:"+channel.GuildID).Str()
				if err != nil {
					channelID = m.ChannelID
				}

				target := parsed.Args[0].DiscordUser()

				err = common.BotSession.GuildBanCreate(channel.GuildID, target.ID, 1)
				if err != nil {
					return err
				}

				common.BotSession.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> Banned **%s** *(%s)*\n%s", m.Author.ID, target.Username, target.ID, parsed.Args[1].Str()))

				log.Println("Banned ", parsed.Args[0].DiscordUser().Username, "cause", parsed.Args[1].Str())

				return nil
			},
		},
	},
	&bot.CustomCommand{
		Key: "moderation_kick_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Kick",
			Description:  "Kicks a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				perms, err := common.BotSession.State.UserChannelPermissions(m.Author.ID, m.ChannelID)
				if err != nil {
					return err
				}

				if !AdminOrPerm(perms, discordgo.PermissionKickMembers) {
					return errors.New("User has no admin or kick permissions")
				}

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				channel, err := common.BotSession.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				channelID, err := client.Cmd("GET", "moderation_action_channel:"+channel.GuildID).Str()
				if err != nil {
					channelID = m.ChannelID
				}

				target := parsed.Args[0].DiscordUser()

				err = common.BotSession.GuildMemberDelete(channel.GuildID, target.ID)
				if err != nil {
					return err
				}

				common.BotSession.ChannelMessageSend(channelID, fmt.Sprintf("<@%s> Kicked **%s** *(%s)*\n%s", m.Author.ID, target.Username, target.ID, parsed.Args[1].Str()))

				log.Println("Kicked ", parsed.Args[0].DiscordUser().Username, "cause", parsed.Args[1].Str())

				return nil
			},
		},
	},
	&bot.CustomCommand{
		Key: "moderation_report_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Report",
			Description:  "Reports a member",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "User", Type: commandsystem.ArgumentTypeUser},
				&commandsystem.ArgumentDef{Name: "Reason", Type: commandsystem.ArgumentTypeString},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {

				client, err := common.RedisPool.Get()
				if err != nil {
					return err
				}
				defer common.RedisPool.Put(client)

				channel, err := common.BotSession.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				// Send typing event to indicate the bot is working
				common.BotSession.ChannelTyping(m.ChannelID)

				logId, err := common.CreatePastebinLog(m.ChannelID)
				if err != nil {
					return errors.New("Failed uploading to pastebin: " + err.Error())
				}

				channelID, err := client.Cmd("GET", "moderation_report_channel:"+channel.GuildID).Str()
				if err != nil || channelID == "" {
					channelID = channel.GuildID
				}
				reportBody := fmt.Sprintf("<@%s> Reported <@%s> For %s\nLast 100 messages from channel: <http://pastebin.com/%s>", m.Author.ID, parsed.Args[0].DiscordUser().ID, parsed.Args[1].Str(), logId)

				_, err = common.BotSession.ChannelMessageSend(channelID, reportBody)
				if err != nil {
					return err
				}

				common.BotSession.ChannelMessageSend(m.ChannelID, "User reported to the proper authorities")

				return nil
			},
		},
	},
	&bot.CustomCommand{
		Key: "moderation_clean_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Clean",
			Description:  "Cleans the chat",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Num", Type: commandsystem.ArgumentTypeNumber},
				&commandsystem.ArgumentDef{Name: "User", Description: "Optionally specify a user, Deletions may be less than `num` if set", Type: commandsystem.ArgumentTypeUser},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				perms, err := common.BotSession.State.UserChannelPermissions(m.Author.ID, m.ChannelID)
				if err != nil {
					return err
				}

				if !AdminOrPerm(perms, discordgo.PermissionManageMessages) {
					return errors.New("User has no admin or manage messages permissions")
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
						return errors.New("Bot is having a stroke <https://www.youtube.com/watch?v=dQw4w9WgXcQ>")
					}
					return errors.New("Can't delete nothing")
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
					if (filter == "" || msgs[i].Author.ID == filter) && msgs[i].ID != m.ID {
						ids = append(ids, msgs[i].ID)
						if len(ids) >= num {
							break
						}
					}
				}
				ids = append(ids, m.ID)

				if len(ids) < 2 {
					common.BotSession.ChannelMessageSend(m.ChannelID, "Deleted nothing... Sorry :'(")
					return nil
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

				return err
			},
		},
	},
}
