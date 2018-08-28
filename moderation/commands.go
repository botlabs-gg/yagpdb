package moderation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
)

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
		channelID := data.CS.ID
		guildID := data.GS.ID

		cmdName := data.Cmd.Trigger.Names[0]

		config, err := GetConfig(guildID)
		if err != nil {
			return "Error retrieving config", err
		}

		enabled := false
		reasonOptional := false
		var requiredRoles []int64

		reasonArgIndex := 1
		switch cmd {
		case ModCmdBan:
			enabled = config.BanEnabled
			reasonOptional = config.BanReasonOptional
			requiredRoles = config.BanCmdRoles
		case ModCmdKick:
			enabled = config.KickEnabled
			reasonOptional = config.KickReasonOptional
			requiredRoles = config.KickCmdRoles
		case ModCmdMute, ModCmdUnMute:
			enabled = config.MuteEnabled
			if cmd == ModCmdMute {
				reasonOptional = config.MuteReasonOptional
				reasonArgIndex = 2
			} else {
				reasonOptional = config.UnmuteReasonOptional
			}
			requiredRoles = config.MuteCmdRoles
		case ModCmdClean:
			reasonOptional = true
			enabled = config.CleanEnabled
			reasonArgIndex = -1
		case ModCmdReport:
			reasonOptional = true
			enabled = config.ReportEnabled
		case ModCmdReason:
			reasonOptional = true
			enabled = true
		case ModCmdWarn:
			reasonOptional = true
			enabled = config.WarnCommandsEnabled
			requiredRoles = config.WarnCmdRoles
		default:
			panic("Unknown command")
		}

		requiredRoleFound := false
		if len(requiredRoles) > 0 {
			// Check if the user has one of the required roles
			member, err := bot.GetMember(guildID, userID)
			if err != nil {
				return "Failed fetching member", err
			}
			for _, r := range member.Roles {
				if common.ContainsInt64Slice(requiredRoles, r) {
					requiredRoleFound = true
					break
				}
			}
		}

		if !requiredRoleFound && neededPerm != 0 {
			// Fallback to legacy permissions
			hasPerms, err := bot.AdminOrPerm(neededPerm, userID, channelID)
			if err != nil || !hasPerms {
				return fmt.Sprintf("The **%s** command requires the **%s** permission in this channel, you don't have it. (if you do contact bot support)", cmdName, common.StringPerms[neededPerm]), nil
			}
		}

		if !enabled {
			return fmt.Sprintf("The **%s** command is disabled on this server. Enable it in the control panel on the moderation page.", cmdName), nil
		}

		if reasonArgIndex != -1 && reasonArgIndex < len(data.Args) {
			reason := SafeArgString(data, reasonArgIndex)
			if !reasonOptional && reason == "" {
				return "Reason is required.", nil
			} else if reason == "" {
				data.Args[reasonArgIndex].Value = "(No reason specified)"
			}
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
		Aliases:       []string{"banid"},
		Description:   "Bans a member, specify a duration with -d",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "d", Default: time.Duration(0), Name: "Duration", Type: &commands.DurationArg{}},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionBanMembers, ModCmdBan, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			reason := SafeArgString(parsed, 1)

			targetID := parsed.Args[0].Int64()
			targetMember := parsed.GS.MemberCopy(true, targetID)
			var target *discordgo.User
			if targetMember == nil || !targetMember.MemberSet {
				target = &discordgo.User{
					Username:      "unknown",
					Discriminator: "????",
					ID:            targetID,
				}
			} else {
				target = targetMember.DGoUser()
			}

			err := BanUserWithDuration(config, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, target, parsed.Switches["d"].Value.(time.Duration), false)
			if err != nil {
				return nil, err
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
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdKick, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			reason := SafeArgString(parsed, 1)

			targetID := parsed.Args[0].Int64()
			targetMember := parsed.GS.MemberCopy(true, targetID)
			var target *discordgo.User
			if targetMember == nil || !targetMember.MemberSet {
				target = &discordgo.User{
					Username:      "unknown",
					Discriminator: "????",
					ID:            targetID,
				}
			} else {
				target = targetMember.DGoUser()
			}

			err := KickUser(config, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, target)
			if err != nil {
				return nil, err
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
			&dcmd.ArgDef{Name: "Duration", Default: time.Minute * 10, Type: &commands.DurationArg{Max: time.Hour * 24 * 7}},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		ArgumentCombos: [][]int{[]int{0, 1, 2}, []int{0, 2, 1}, []int{0, 1}, []int{0, 2}, []int{0}},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdMute, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)
			if config.MuteRole == "" {
				return "No mute role set up, assign a mute role in the control panel", nil
			}

			target := parsed.Args[0].Value.(*discordgo.User)
			muteDuration := int(parsed.Args[1].Value.(time.Duration).Minutes())
			reason := parsed.Args[2].Str()

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, true, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, muteDuration)
			if err != nil {
				return nil, err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Unmute",
		Description:   "Unmutes a member",
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

			target := parsed.Args[0].Value.(*discordgo.User)
			reason := parsed.Args[1].Str()

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, false, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, 0)
			if err != nil {
				return nil, err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		Cooldown:      5,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Report",
		Description:   "Reports a member to the server's staff",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(0, ModCmdReport, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			logLink := CreateLogs(parsed.GS.ID, parsed.CS.ID, parsed.Msg.Author)

			channelID := config.IntReportChannel()
			if channelID == 0 {
				return "No report channel set up", nil
			}

			reportBody := fmt.Sprintf("<@%d> Reported <@%d> in <#%d> For `%s`\nLast 100 messages from channel: <%s>", parsed.Msg.Author.ID, parsed.Args[0].Value.(*discordgo.User).ID, parsed.Msg.ChannelID, parsed.Args[1].Str(), logLink)

			_, err := common.BotSession.ChannelMessageSend(channelID, common.EscapeSpecialMentions(reportBody))
			if err != nil {
				return nil, err
			}

			// don't bother sending confirmation if it's in the same channel
			if channelID != parsed.Msg.ChannelID {
				return "User reported to the proper authorities", nil
			}
			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled:   true,
		CmdCategory:     commands.CategoryModeration,
		Name:            "Clean",
		Description:     "Delete the last number of messages from chat, optionally filtering by user, max age and regex.",
		LongDescription: "Specify a regex with \"-r regex_here\" and max age with \"-ma 1h10m\"\nNote: Will only look in the last 1k messages",
		Aliases:         []string{"clear", "cl"},
		RequiredArgs:    1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Num", Type: &dcmd.IntArg{Min: 1, Max: 100}},
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "r", Name: "Regex", Type: dcmd.String},
			&dcmd.ArgDef{Switch: "ma", Default: time.Duration(0), Name: "Max age", Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Switch: "i", Name: "Regex case insensitive"},
		},
		ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdClean, func(parsed *dcmd.Data) (interface{}, error) {
			var userFilter int64
			if parsed.Args[1].Value != nil {
				userFilter = parsed.Args[1].Value.(*discordgo.User).ID
			}

			num := parsed.Args[0].Int()
			if userFilter == 0 || userFilter == parsed.Msg.Author.ID {
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

			filtered := false

			// Check if we should regex match this
			re := ""
			if parsed.Switches["r"].Value != nil {
				filtered = true
				re = parsed.Switches["r"].Str()

				// Add the case insensitive flag if needed
				if parsed.Switches["i"].Value != nil && parsed.Switches["i"].Value.(bool) {
					if !strings.HasPrefix(re, "(?i)") {
						re = "(?i)" + re
					}
				}
			}

			// Check if we have a max age
			ma := parsed.Switches["ma"].Value.(time.Duration)
			if ma != 0 {
				filtered = true
			}

			limitFetch := num
			if userFilter != 0 || filtered {
				limitFetch = num * 50 // Maybe just change to full fetch?
			}

			if limitFetch > 1000 {
				limitFetch = 1000
			}

			// Wait a second so the client dosen't gltich out
			time.Sleep(time.Second)

			numDeleted, err := AdvancedDeleteMessages(parsed.Msg.ChannelID, userFilter, re, ma, num, limitFetch)

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
			&dcmd.ArgDef{Name: "ID", Type: dcmd.Int},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionKickMembers, ModCmdReason, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)
			if config.ActionChannel == "" {
				return "No mod log channel set up", nil
			}
			msg, err := common.BotSession.ChannelMessage(config.IntActionChannel(), parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			if msg.Author.ID != common.Conf.BotID {
				return "I didn't make that message", nil
			}

			if len(msg.Embeds) < 1 {
				return "This entry is either too old or you're trying to mess with me...", nil
			}

			embed := msg.Embeds[0]
			updateEmbedReason(parsed.Msg.Author, parsed.Args[1].Str(), embed)
			_, err = common.BotSession.ChannelMessageEditEmbed(config.IntActionChannel(), msg.ID, embed)
			if err != nil {
				return nil, err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Warn",
		Description:   "Warns a user, warnings are saved using the bot. Use -warnings to view them.",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserReqMention},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {
			config := parsed.Context().Value(ContextKeyConfig).(*Config)

			err := WarnUser(config, parsed.GS.ID, parsed.CS.ID, parsed.Msg.Author, parsed.Args[0].Value.(*discordgo.User), parsed.Args[1].Str())
			if err != nil {
				return nil, err
			}

			return "👌", nil
		}),
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Warnings",
		Description:   "Lists warning of a user.",
		Aliases:       []string{"Warns"},
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {

			userID := parsed.Args[0].Int64()

			var result []*WarningModel
			err := common.GORM.Where("user_id = ? AND guild_id = ?", userID, parsed.GS.ID).Order("id desc").Find(&result).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return nil, err
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

			rows := common.GORM.Model(WarningModel{}).Where("guild_id = ? AND id = ?", parsed.GS.ID, parsed.Args[0].Int()).Update(
				"message", fmt.Sprintf("%s (updated by %s#%s (%d))", parsed.Args[1].Str(), parsed.Msg.Author.Username, parsed.Msg.Author.Discriminator, parsed.Msg.Author.ID)).RowsAffected

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

			rows := common.GORM.Where("guild_id = ? AND id = ?", parsed.GS.ID, parsed.Args[0].Int()).Delete(WarningModel{}).RowsAffected
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
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
		},
		RunFunc: ModBaseCmd(discordgo.PermissionManageMessages, ModCmdWarn, func(parsed *dcmd.Data) (interface{}, error) {

			userID := parsed.Args[0].Int64()

			rows := common.GORM.Where("guild_id = ? AND user_id = ?", parsed.GS.ID, userID).Delete(WarningModel{}).RowsAffected
			return fmt.Sprintf("Deleted %d warnings.", rows), nil
		}),
	},
}

func AdvancedDeleteMessages(channelID int64, filterUser int64, regex string, maxAge time.Duration, deleteNum, fetchNum int) (int, error) {
	var compiledRegex *regexp.Regexp
	if regex != "" {
		// Start by compiling the regex
		var err error
		compiledRegex, err = regexp.Compile(regex)
		if err != nil {
			return 0, err
		}
	}

	msgs, err := bot.GetMessages(channelID, fetchNum, false)
	if err != nil {
		return 0, err
	}

	toDelete := make([]int64, 0)
	now := time.Now()
	for i := len(msgs) - 1; i >= 0; i-- {
		if filterUser != 0 && msgs[i].Author.ID != filterUser {
			continue
		}

		parsedCreatedAt, _ := msgs[i].Timestamp.Parse()
		// Can only bulk delete messages up to 2 weeks (but add 1 minute buffer account for time sync issues and other smallies)
		if now.Sub(parsedCreatedAt) > (time.Hour*24*14)-time.Minute {
			continue
		}

		// Check regex
		if compiledRegex != nil {
			if !compiledRegex.MatchString(msgs[i].Content) {
				continue
			}
		}

		// Check max age
		if maxAge != 0 && now.Sub(parsedCreatedAt) > maxAge {
			continue
		}

		toDelete = append(toDelete, msgs[i].ID)
		//log.Println("Deleting", msgs[i].ContentWithMentionsReplaced())
		if len(toDelete) >= deleteNum || len(toDelete) >= 100 {
			break
		}
	}

	if len(toDelete) < 1 {
		return 0, nil
	}

	if len(toDelete) < 1 {
		return 0, nil
	} else if len(toDelete) == 1 {
		err = common.BotSession.ChannelMessageDelete(channelID, toDelete[0])
	} else {
		err = common.BotSession.ChannelMessagesBulkDelete(channelID, toDelete)
	}

	return len(toDelete), err
}
