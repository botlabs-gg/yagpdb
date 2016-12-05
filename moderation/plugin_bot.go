package moderation

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs"
	"strings"
	"sync"
	"time"
)

var (
	ErrFailedPerms = errors.New("Failed retrieving perms")
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(ModerationCommands...)
	common.BotSession.AddHandler(bot.CustomGuildBanRemove(HandleGuildBanRemove))
}

func HandleGuildBanRemove(s *discordgo.Session, r *discordgo.GuildBanRemove, client *redis.Client) {
	config, err := GetConfig(r.GuildID)
	if err != nil {
		logrus.WithError(err).Error("Failed retrieving config")
		return
	}

	if !config.LogUnbans || config.ActionChannel == "" {
		return
	}

	embed := CreateModlogEmbed(nil, "Unbanned", r.User, "", "")
	_, err = s.ChannelMessageSendEmbed(config.ActionChannel, embed)
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
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Ban",
			Description:  "Bans a member",
			RequiredArgs: 1,
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

			err = BanUser(config, parsed.Guild.ID, m.ChannelID, m.Author, reason, target)
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
			RequiredArgs: 1,
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

			err = KickUser(config, parsed.Guild.ID, m.ChannelID, m.Author, reason, target)
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
			RequiredArgs: 2,
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

			muteDuration := parsed.Args[1].Int()
			if muteDuration < 1 || muteDuration > 1440 {
				return "Duration out of bounds (min 1, max 1440 - 1 day)", nil
			}

			target := parsed.Args[0].DiscordUser()

			member, err := common.BotSession.State.Member(parsed.Guild.ID, target.ID)
			if err != nil {
				return "I COULDNT FIND ZE GUILDMEMEBER PLS HELP AAAAAAA", err
			}

			err = MuteUnmuteUser(config, client, true, parsed.Guild.ID, m.ChannelID, m.Author, reason, member, parsed.Args[1].Int())
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
			RequiredArgs: 1,
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

			member, err := common.BotSession.State.Member(parsed.Guild.ID, target.ID)
			if err != nil {
				return "I COULDNT FIND ZE GUILDMEMEBER PLS HELP AAAAAAA", err
			}

			err = MuteUnmuteUser(config, client, false, parsed.Guild.ID, m.ChannelID, m.Author, reason, member, 0)
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
			config, _, err := BaseCmd(0, m.Author.ID, m.ChannelID, parsed.Guild.ID)
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
	&commands.CustomCommand{
		Cooldown: 5,
		Category: commands.CategoryModeration,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "SearchModLog",
			Aliases:      []string{"sml"},
			Description:  "Searches the mod log up to 'Msgs' messages back for a string",
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Msgs", Type: commandsystem.ArgumentTypeNumber},
				&commandsystem.ArgumentDef{Name: "What", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			conf, perm, err := BaseCmd(discordgo.PermissionKickMembers, m.Author.ID, m.ChannelID, parsed.Guild.ID)
			if err != nil {
				return "Error retrieving config.", err
			}
			if !perm {
				return "You do not have manage messages permissions in this channel.", nil
			}

			num := parsed.Args[0].Int()
			if num > 100000 {
				num = 100000
			}

			if num < 1 {
				if num < 0 {
					return errors.New("Bot is having a stroke <https://www.youtube.com/watch?v=dQw4w9WgXcQ>"), nil
				}
				return errors.New("Can't delete nothing"), nil
			}

			currentSearchingLock.Lock()
			if remaining, ok := currentSearching[parsed.Guild.ID]; ok {
				currentSearchingLock.Unlock()
				return fmt.Sprintf("Already searching a channel in this server, ETA: %s", common.HumanizeDuration(common.DurationPrecisionSeconds, time.Duration(remaining/100)*time.Second)), nil
			}
			currentSearching[parsed.Guild.ID] = num
			currentSearchingLock.Unlock()

			privChannel, err := bot.GetCreatePrivateChannel(common.BotSession, m.Author.ID)
			if err != nil {
				return "Failed retrieving private channel", err
			}
			go searcher(parsed.Guild.ID, conf.ActionChannel, privChannel.ID, num, strings.ToLower(parsed.Args[1].Str()))

			return fmt.Sprintf("Started searching back %d messages, ETA: %s, i will send you a DM with results.", num, common.HumanizeDuration(common.DurationPrecisionSeconds, time.Duration(num/100)*time.Second)), err
		},
	},
	&commands.CustomCommand{
		Cooldown: 5,
		Category: commands.CategoryModeration,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "SearchStatus",
			Aliases:     []string{"ss"},
			Description: "Responds with the status of the current search going on",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			currentSearchingLock.Lock()
			defer currentSearchingLock.Unlock()
			if remaining, ok := currentSearching[parsed.Guild.ID]; ok {
				return fmt.Sprintf("A search in this server is going on, ETA: %s", common.HumanizeDuration(common.DurationPrecisionSeconds, time.Duration(remaining/100)*time.Second)), nil
			}

			return "No search is currently ongoign in this server", nil
		},
	},
}

var (
	currentSearching     = make(map[string]int)
	currentSearchingLock sync.Mutex
)

func searcher(guildID, channelID string, sendTo string, limit int, what string) {
	before := ""

	remaining := limit

	errMsg := ""

	embedHits := make([]*SearchHit, 0)
	normalHits := ""

	numRetries := 0
	for {
		numFetching := remaining
		if numFetching > 100 {
			numFetching = 100
		}

		msgs, err := common.BotSession.ChannelMessages(channelID, numFetching, before, "")
		if err != nil {
			// If it was a api error then we must be msising permissions or something, either way we can't continue
			if cast, ok := err.(*discordgo.RESTError); ok {
				errMsg = "Unknown error"
				if cast.Message != nil {
					errMsg = cast.Message.Message
				}
				break
			}

			// Otherwise retry up to 5 times
			numRetries++
			if numRetries > 5 {
				logrus.WithError(err).Error("Stopped search early")
				errMsg = "Network error?"
				break
			}
			continue
		}

		for _, v := range msgs {
			if hit := searchMsg(v, what); hit != nil {
				parsedTime, err := v.Timestamp.Parse()
				if err != nil {
					logrus.WithError(err).Error("Failed parsing timestamp")
					continue
				}

				prefix := fmt.Sprintf("%s (ID %s): ", parsedTime.UTC().Format(time.RFC822), v.ID)

				if hit.Embed != nil {
					hit.Embed.Title = prefix + hit.Embed.Title
					embedHits = append(embedHits, hit)
				} else {
					normalHits += prefix + hit.Content + "\n\n"
				}
			}
		}
		remaining -= numFetching
		logrus.Println(len(msgs), numFetching)
		// were done here, fucking quit life
		if remaining < 1 || len(msgs) < numFetching {
			break
		} else {
			before = msgs[len(msgs)-1].ID
			currentSearchingLock.Lock()
			currentSearching[guildID] = remaining
			currentSearchingLock.Unlock()
		}
	}

	if normalHits != "" {

		_, err := dutil.SplitSendMessage(common.BotSession, sendTo, normalHits)
		if err != nil {
			errMsg += "\nFailed sending some replies"
			logrus.WithError(err).Error("Failed sending search replies")
		}
	}

	for _, hit := range embedHits {
		_, err := common.BotSession.ChannelMessageSendEmbed(sendTo, hit.Embed)
		if err != nil {
			errMsg += "\nFailed sending some replies"
			logrus.WithError(err).Error("Failed sending search results")
			break
		}
	}

	extraMsg := ""
	if normalHits == "" && len(embedHits) < 1 {
		extraMsg = "No hits for searchstring: " + what + "\n"
	} else {
		extraMsg = "All results sent."
	}
	if errMsg != "" {
		extraMsg += "Errors:\n" + errMsg
	}
	_, err := common.BotSession.ChannelMessageSend(sendTo, extraMsg)
	if err != nil {
		logrus.WithError(err).Error("Failed sending final reply")
	}

	currentSearchingLock.Lock()
	delete(currentSearching, guildID)
	currentSearchingLock.Unlock()
}

type SearchHit struct {
	Embed   *discordgo.MessageEmbed
	Content string
}

func searchMsg(m *discordgo.Message, searchStr string) *SearchHit {
	if strings.Contains(strings.ToLower(m.Content), searchStr) {
		return &SearchHit{
			Content: m.Content,
		}
	}

	for _, embed := range m.Embeds {
		if embed.Type != "rich" {
			continue
		}

		if strings.Contains(strings.ToLower(embed.Title), searchStr) || strings.Contains(strings.ToLower(embed.Description), searchStr) {
			return &SearchHit{
				Embed: embed,
			}
		}
		if embed.Footer != nil {
			if strings.Contains(strings.ToLower(embed.Footer.Text), searchStr) {
				return &SearchHit{
					Embed: embed,
				}
			}
		}

		for _, field := range embed.Fields {
			if strings.Contains(strings.ToLower(field.Name), searchStr) || strings.Contains(strings.ToLower(field.Value), searchStr) {
				return &SearchHit{
					Embed: embed,
				}
			}
		}
	}

	return nil
}
