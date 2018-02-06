package moderation

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
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
	commands.AddRootCommands(ModerationCommands...)
	eventsystem.AddHandler(HandleGuildBanAddRemove, eventsystem.EventGuildBanRemove)
	eventsystem.AddHandler(bot.RedisWrapper(HandleGuildBanAddRemove), eventsystem.EventGuildBanAdd)
	eventsystem.AddHandler(bot.RedisWrapper(HandleMemberJoin), eventsystem.EventGuildMemberAdd)
}

func HandleGuildBanAddRemove(evt *eventsystem.EventData) {
	var user *discordgo.User
	guildID := ""
	action := ""

	switch evt.Type {
	case eventsystem.EventGuildBanAdd:

		guildID = evt.GuildBanAdd.GuildID
		user = evt.GuildBanAdd.User
		action = ActionBanned
		if i, _ := bot.ContextRedis(evt.Context()).Cmd("GET", RedisKeyBannedUser(guildID, user.ID)).Int(); i > 0 {
			bot.ContextRedis(evt.Context()).Cmd("DEL", RedisKeyBannedUser(guildID, user.ID))
			return
		}
	case eventsystem.EventGuildBanRemove:
		action = ActionUnbanned
		user = evt.GuildBanRemove.User
		guildID = evt.GuildBanRemove.GuildID
	default:
		return
	}

	config, err := GetConfig(guildID)
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("Failed retrieving config")
		return
	}

	if config.ActionChannel == "" {
		return
	}

	if (action == ActionUnbanned && !config.LogUnbans) || (action == ActionBanned && !config.LogBans) {
		return
	}

	err = CreateModlogEmbed(config.ActionChannel, nil, action, user, "", "")
	if err != nil {
		logrus.WithError(err).WithField("guild", guildID).Error("Failed sending " + action + " log message")
	}
}

func HandleMemberJoin(evt *eventsystem.EventData) {
	c := evt.GuildMemberAdd
	client := bot.ContextRedis(evt.Context())

	muteLeft, _ := client.Cmd("TTL", RedisKeyMutedUser(c.GuildID, c.User.ID)).Int()
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
	ModCmdReason
	ModCmdWarn
)

// ModBaseCmd is the base command for moderation commands, it makes sure proper permissions are there for the user invoking it
// and that the command is required and the reason is specified if required
func ModBaseCmd(neededPerm, cmd int, inner dcmd.RunFunc) dcmd.RunFunc {
	// userID, channelID, guildID string (config *Config, hasPerms bool, err error) {

	return func(data *dcmd.Data) (interface{}, error) {

		userID := data.Msg.Author.ID
		channelID := data.CS.ID()
		guildID := data.GS.ID()

		cmdName := data.Cmd.Trigger.Names[0]

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
			reason = SafeArgString(data, 1)
		case ModCmdKick:
			enabled = config.KickEnabled
			reasonOptional = config.KickReasonOptional
			reason = SafeArgString(data, 1)
		case ModCmdMute, ModCmdUnMute:
			enabled = config.MuteEnabled
			if cmd == ModCmdMute {
				reasonOptional = config.MuteReasonOptional
				reason = SafeArgString(data, 2)
			} else {
				reasonOptional = config.UnmuteReasonOptional
				reason = SafeArgString(data, 1)
			}
		case ModCmdClean:
			reasonOptional = true
			enabled = config.CleanEnabled
		case ModCmdReport:
			reasonOptional = true
			enabled = config.ReportEnabled
		case ModCmdReason:
			reasonOptional = true
			enabled = true
		case ModCmdWarn:
			reasonOptional = true
			enabled = config.WarnCommandsEnabled
		default:
			panic("Unknown command")
		}

		if !enabled {
			return fmt.Sprintf("The **%s** command is disabled on this server. Enable it in the control panel on the moderation page.", cmdName), nil
		}

		if !reasonOptional && reason == "" {
			return "Reason is required.", nil
		}

		return inner(data.WithContext(context.WithValue(data.Context(), ContextKeyConfig, config)))

	}
}

func SafeArgString(data *dcmd.Data, arg int) string {
	if arg >= len(data.Args) || data.Args[arg].Value == nil {
		return ""
	}

	return data.Args[arg].Str()
}

var ModerationCommands = []*commands.YAGCommand{
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Ban",
		Description:   "Bans a member",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionBanMembers, ModCmdBan, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			reason := SafeArgString(parsed, 1)
			if reason == "" {
				reason = "(No reason specified)"
			}

			target := parsed.Args[0].Value.(*discordgo.User)

			err := BanUser(parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), config, parsed.GS.ID(), parsed.Msg.ChannelID, parsed.Msg.Author, reason, target)
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return cast.Message.Message, err
				} else {
					return "An error occurred", err
				}
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Kick",
		Description:   "Kicks a member",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdKick, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			reason := SafeArgString(parsed, 1)
			if reason == "" {
				reason = "(No reason specified)"
			}

			target := parsed.Args[0].Value.(*discordgo.User)

			err := KickUser(config, parsed.GS.ID(), parsed.Msg.ChannelID, parsed.Msg.Author, reason, target)
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return cast.Message.Message, err
				} else {
					return "An error occurred", err
				}
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Mute",
		Description:   "Mutes a member",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Minutes", Type: &dcmd.IntArg{Min: 1, Max: 1440}},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		ArgumentCombos: [][]int{[]int{0, 1, 2}, []int{0, 1}, []int{0, 2}, []int{0}},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdMute, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)
			if config.MuteRole == "" {
				return "No mute role set up, assign a mute role in the control panel", nil
			}

			reason := "(No reason specified)"
			if parsed.Args[2].Str() != "" {
				reason = parsed.Args[2].Str()
			}

			muteDuration := 10
			if parsed.Args[1].Value != nil {
				muteDuration = parsed.Args[1].Int()
				if muteDuration < 1 || muteDuration > 1440 {
					return "Duration out of bounds (min 1, max 1440 - 1 day)", nil
				}
			}

			target := parsed.Args[0].Value.(*discordgo.User)
			member, err := bot.GetMember(parsed.GS.ID(), target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), true, parsed.GS.ID(), parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, muteDuration)
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return "API Error: " + cast.Message.Message, err
				} else {
					return "An error occurred", err
				}
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Unmute",
		Description:   "unmutes a member",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdUnMute, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)
			if config.MuteRole == "" {
				return "No mute role set up, assign a mute role in the control panel", nil
			}

			reason := "(No reason specified)"
			if parsed.Args[1].Str() != "" {
				reason = parsed.Args[1].Str()
			} else if !config.UnmuteReasonOptional {
				return "No reason specified", nil
			}

			target := parsed.Args[0].Value.(*discordgo.User)
			member, err := bot.GetMember(parsed.GS.ID(), target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client), false, parsed.GS.ID(), parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, 0)
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return "API Error: " + cast.Message.Message, err
				} else {
					return "An error occurred", err
				}
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		Cooldown:      5,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Report",
		Description:   "Reports a member",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(0, ModCmdReport, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			logLink := CreateLogs(parsed.GS.ID(), parsed.CS.ID(), parsed.Msg.Author)

			channelID := config.ReportChannel
			if channelID == "" {
				return "No report channel set up", nil
			}

			reportBody := fmt.Sprintf("<@%s> Reported <@%s> in <#%s> For `%s`\nLast 100 messages from channel: <%s>", parsed.Msg.Author.ID, parsed.Args[0].Value.(*discordgo.User).ID, parsed.Msg.ChannelID, parsed.Args[1].Str(), logLink)

			_, err := common.BotSession.ChannelMessageSend(channelID, common.EscapeSpecialMentions(reportBody))
			if err != nil {
				return "Failed sending report, check perms for report channel", err
			}

			// don't bother sending confirmation if it's in the same channel
			if channelID != parsed.Msg.ChannelID {
				return "User reported to the proper authorities", nil
			}
			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Clean",
		Description:   "Deleted the last n messages from chat, optionally filtering by user",
		Aliases:       []string{"clear", "cl"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Num", Type: &dcmd.IntArg{Min: 1, Max: 100}},
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
		},
		ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdClean, func(parsed *dcmd.Data) (interface{}, error) {
			filter := ""
			if parsed.Args[1].Value != nil {
				filter = parsed.Args[1].Value.(*discordgo.User).ID
			}

			num := parsed.Args[0].Int()
			if filter == "" || filter == parsed.Msg.Author.ID {
				num++ // Automatically include our own message
			}

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

			numDeleted, err := DeleteMessages(parsed.Msg.ChannelID, filter, num, limitFetch)

			return dcmd.NewTemporaryResponse(time.Second*5, fmt.Sprintf("Deleted %d message(s)! :')", numDeleted), true), err
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Reason",
		Description:   "Add/Edit a modlog reason",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "ID", Type: dcmd.String},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdReason, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)
			if config.ActionChannel == "" {
				return "No mod log channel set up", nil
			}
			msg, err := common.BotSession.ChannelMessage(config.ActionChannel, parsed.Args[0].Str())
			if err != nil {
				if cast, ok := err.(*discordgo.RESTError); ok && cast.Message != nil {
					return "Failed retrieving the message: " + cast.Message.Message, nil
				}
				return "Failed retrieving the message", err
			}

			if msg.Author.ID != common.Conf.BotID {
				return "I didn't make that message", nil
			}

			if len(msg.Embeds) < 1 {
				return "This entry is either too old or you're trying to mess with me...", nil
			}

			embed := msg.Embeds[0]
			updateEmbedReason(parsed.Msg.Author, parsed.Args[1].Str(), embed)
			_, err = common.BotSession.ChannelMessageEditEmbed(config.ActionChannel, msg.ID, embed)
			if err != nil {
				return "Failed updating the modlog entry", err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Warn",
		Description:   "Warn a user, warning are saved.",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			err := WarnUser(config, parsed.GS.ID(), parsed.CS.ID(), parsed.Msg.Author, parsed.Args[0].Value.(*discordgo.User), parsed.Args[1].Str())
			if err != nil {
				return "Seomthing went wrong warning this user, make sure the bot has all the proper perms. (if you have the modlog enabled the bot need to be able to send messages in the modlog for example)", err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Warnings",
		Description:   "Lists warning of a user.",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {
			var result []*WarningModel
			err := common.GORM.Where("user_id = ? AND guild_id = ?", parsed.Args[0].Value.(*discordgo.User).ID, parsed.GS.ID()).Order("id desc").Find(&result).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return "An error occured...", err
			}

			if len(result) < 1 {
				return "This user has not received any warnings", nil
			}

			out := ""
			for _, entry := range result {
				out += fmt.Sprintf("#%d: `%20s` **%s** (%13s) - **%s**\n", entry.ID, entry.CreatedAt.Format(time.RFC822), entry.AuthorUsernameDiscrim, entry.AuthorID, entry.Message)
				if entry.LogsLink != "" {
					out += "^logs: <" + entry.LogsLink + ">\n"
				}
			}

			return out, nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "EditWarning",
		Description:   "Edit a warning, id is the first number of each warning from the warnings command",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Id", Type: dcmd.Int},
			&dcmd.ArgDef{Name: "NewMessage", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {

			rows := common.GORM.Model(WarningModel{}).Where("guild_id = ? AND id = ?", parsed.GS.ID(), parsed.Args[0].Int()).Update(
				"message", fmt.Sprintf("%s (updated by %s#%s (%s))", parsed.Args[1].Str(), parsed.Msg.Author.Username, parsed.Msg.Author.Discriminator, parsed.Msg.Author.ID)).RowsAffected

			if rows < 1 {
				return "Failed updating, most likely couldn't find the warning", nil
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "DelWarning",
		Aliases:       []string{"dw"},
		Description:   "Deletes a warning, id is the first number of each warning from the warnings command",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Id", Type: dcmd.Int},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {

			rows := common.GORM.Where("guild_id = ? AND id = ?", parsed.GS.ID(), parsed.Args[0].Int()).Delete(WarningModel{}).RowsAffected
			if rows < 1 {
				return "Failed deleting, most likely couldn't find the warning", nil
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "ClearWarnings",
		Aliases:       []string{"clw"},
		Description:   "Clears the warnings of a user",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {

			rows := common.GORM.Where("guild_id = ? AND user_id = ?", parsed.GS.ID(), parsed.Args[0].Value.(*discordgo.User).ID).Delete(WarningModel{}).RowsAffected
			return fmt.Sprintf("Deleted %d warnings.", rows), nil
		}),
	},
}
