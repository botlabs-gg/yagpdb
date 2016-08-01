package moderation

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"log"
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
			Name:         "ban",
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
					return errors.New("Neither admin or has ban permission")
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
			Name:         "kick",
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
					return errors.New("Neither admin or has kick permission")
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
			Name:         "report",
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

				channelID, err := client.Cmd("GET", "moderation_report_channel:"+channel.GuildID).Str()
				if err != nil {
					channelID = channel.GuildID
				}

				reportBody := fmt.Sprintf("<@%s> Reported <@%s> For %s", m.Author.ID, parsed.Args[0].DiscordUser().ID, parsed.Args[1].Str())

				_, err = common.BotSession.ChannelMessageSend(channelID, reportBody)
				if err != nil {
					return err
				}

				common.BotSession.ChannelMessageSend(m.ChannelID, "User reported to the proper authoratives")

				return nil
			},
		},
	},
	&bot.CustomCommand{
		Key: "moderation_clean_enabled:",
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "clean",
			Description:  "Cleans the chat",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Num", Type: commandsystem.ArgumentTypeNumber},
				&commandsystem.ArgumentDef{Name: "User", Description: "Optionally specify a user", Type: commandsystem.ArgumentTypeUser},
			},
			RunFunc: func(parsed *commandsystem.ParsedCommand, source commandsystem.CommandSource, m *discordgo.MessageCreate) error {
				perms, err := common.BotSession.State.UserChannelPermissions(m.Author.ID, m.ChannelID)
				if err != nil {
					return err
				}

				if !AdminOrPerm(perms, discordgo.PermissionManageMessages) {
					return errors.New("Neither admin or has manage messages permission")
				}

				log.Println("Should clean ", parsed.Args[0].Int(), "msgs")

				channel, err := common.BotSession.State.Channel(m.ChannelID)
				if err != nil {
					return err
				}

				max := parsed.Args[0].Int()

				filter := ""
				if parsed.Args[1] != nil {
					filter = parsed.Args[1].DiscordUser().ID
				}

				bot.Session.State.RLock()
				defer bot.Session.State.RUnlock()

				ids := make([]string, 0)
				for i := len(channel.Messages) - 1; i >= 0; i-- {
					if filter == "" || channel.Messages[i].Author.ID == filter {
						ids = append(ids, channel.Messages[i].ID)
						if len(ids) >= max {
							break
						}
					}
				}

				if len(ids) < 1 {
					common.BotSession.ChannelMessageSend(m.ChannelID, "Deleted nothing... Sorry :'(")
					return nil
				}

				if len(ids) == 1 {
					err = common.BotSession.ChannelMessageDelete(m.ChannelID, ids[0])
				} else {
					err = common.BotSession.ChannelMessagesBulkDelete(m.ChannelID, ids)
				}
				if err == nil {
					common.BotSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Deleted %d messages! :')", len(ids)))
				}
				return err
			},
		},
	},
}
