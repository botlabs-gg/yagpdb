package moderation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/pkg/errors"
)

func MBaseCmd(cmdData *dcmd.Data, targetID int64) (config *Config, targetUser *discordgo.User, err error) {
	config, err = GetConfig(cmdData.GS.ID)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "GetConfig")
	}

	if targetID != 0 {
		targetMember, _ := bot.GetMember(cmdData.GS.ID, targetID)
		if targetMember != nil {
			authorMember := commands.ContextMS(cmdData.Context())
			gs := cmdData.GS

			gs.RLock()
			above := bot.IsMemberAbove(gs, authorMember, targetMember)
			gs.RUnlock()

			if !above {
				return config, targetMember.DGoUser(), commands.NewPublicError("Can't use moderation commands on users ranked the same or higher than you")
			}

			return config, targetMember.DGoUser(), nil
		}
	}

	return config, &discordgo.User{
		Username:      "unknown",
		Discriminator: "????",
		ID:            targetID,
	}, nil

}

func MBaseCmdSecond(cmdData *dcmd.Data, reason string, reasonArgOptional bool, neededPerm int, additionalPermRoles []int64, enabled bool) (oreason string, err error) {
	cmdName := cmdData.Cmd.Trigger.Names[0]
	oreason = reason
	if !enabled {
		return oreason, commands.NewPublicErrorF("The **%s** command is disabled on this server. Enable it in the control panel on the moderation page.", cmdName)
	}

	if strings.TrimSpace(reason) == "" {
		if !reasonArgOptional {
			return oreason, commands.NewPublicError("A reason has been set to be required for this command by the server admins, see help for more info.")
		}

		oreason = "(No reason specified)"
	}

	// check permissions or role setup for this command
	permsMet := false
	if len(additionalPermRoles) > 0 {
		// Check if the user has one of the required roles
		member := commands.ContextMS(cmdData.Context())
		for _, r := range member.Roles {
			if common.ContainsInt64Slice(additionalPermRoles, r) {
				permsMet = true
				break
			}
		}
	}

	if !permsMet && neededPerm != 0 {
		// Fallback to legacy permissions
		hasPerms, err := bot.AdminOrPermMS(commands.ContextMS(cmdData.Context()), cmdData.CS.ID, neededPerm)
		if err != nil || !hasPerms {
			return oreason, commands.NewPublicErrorF("The **%s** command requires the **%s** permission in this channel or additional roles set up by admins, you don't have it. (if you do contact bot support)", cmdName, common.StringPerms[neededPerm])
		}

		permsMet = true
	}

	return oreason, nil
}

func SafeArgString(data *dcmd.Data, arg int) string {
	if arg >= len(data.Args) || data.Args[arg].Value == nil {
		return ""
	}

	return data.Args[arg].Str()
}

func GenericCmdResp(action ModlogAction, target *discordgo.User, duration time.Duration, zeroDurPermanent bool, noDur bool) string {
	durStr := " indefinitely"
	if duration > 0 || !zeroDurPermanent {
		durStr = " for `" + common.HumanizeDuration(common.DurationPrecisionMinutes, duration) + "`"
	}
	if noDur {
		durStr = ""
	}

	userStr := target.Username + "#" + target.Discriminator
	if target.Discriminator == "????" {
		userStr = strconv.FormatInt(target.ID, 10)
	}

	return fmt.Sprintf("%s %s `%s`%s", action.Emoji, action.Prefix, userStr, durStr)
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			reason := SafeArgString(parsed, 1)
			reason, err = MBaseCmdSecond(parsed, reason, config.BanReasonOptional, discordgo.PermissionBanMembers, config.BanCmdRoles, config.BanEnabled)
			if err != nil {
				return nil, err
			}

			err = BanUserWithDuration(config, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, target, parsed.Switches["d"].Value.(time.Duration))
			if err != nil {
				return nil, err
			}

			return GenericCmdResp(MABanned, target, parsed.Switch("d").Value.(time.Duration), true, false), nil
		},
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			reason := SafeArgString(parsed, 1)
			reason, err = MBaseCmdSecond(parsed, reason, config.KickReasonOptional, discordgo.PermissionKickMembers, config.KickCmdRoles, config.KickEnabled)
			if err != nil {
				return nil, err
			}

			err = KickUser(config, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, target)
			if err != nil {
				return nil, err
			}

			return GenericCmdResp(MAKick, target, 0, true, true), nil
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Mute",
		Description:   "Mutes a member",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Duration", Default: time.Minute * 10, Type: &commands.DurationArg{Max: time.Hour * 24 * 7}},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		ArgumentCombos: [][]int{[]int{0, 1, 2}, []int{0, 2, 1}, []int{0, 1}, []int{0, 2}, []int{0}},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			if config.MuteRole == "" {
				return "No mute role set up, assign a mute role in the control panel", nil
			}

			reason := parsed.Args[2].Str()
			reason, err = MBaseCmdSecond(parsed, reason, config.MuteReasonOptional, discordgo.PermissionKickMembers, config.MuteCmdRoles, config.MuteEnabled)
			if err != nil {
				return nil, err
			}

			muteDuration := int(parsed.Args[1].Value.(time.Duration).Minutes())

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, true, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, muteDuration)
			if err != nil {
				return nil, err
			}

			return GenericCmdResp(MAMute, target, time.Duration(muteDuration)*time.Minute, false, false), nil
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Unmute",
		Description:   "Unmutes a member",
		RequiredArgs:  1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			if config.MuteRole == "" {
				return "No mute role set up, assign a mute role in the control panel", nil
			}

			reason := parsed.Args[1].Str()
			reason, err = MBaseCmdSecond(parsed, reason, config.UnmuteReasonOptional, discordgo.PermissionKickMembers, config.MuteCmdRoles, config.MuteEnabled)
			if err != nil {
				return nil, err
			}

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			err = MuteUnmuteUser(config, false, parsed.GS.ID, parsed.Msg.ChannelID, parsed.Msg.Author, reason, member, 0)
			if err != nil {
				return nil, err
			}

			return GenericCmdResp(MAUnmute, target, 0, false, true), nil
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		Cooldown:      5,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Report",
		Description:   "Reports a member to the server's staff",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, 0, nil, config.ReportEnabled)
			if err != nil {
				return nil, err
			}

			target := parsed.Args[0].Int64()

			logLink := CreateLogs(parsed.GS.ID, parsed.CS.ID, parsed.Msg.Author)

			channelID := config.IntReportChannel()
			if channelID == 0 {
				return "No report channel set up", nil
			}

			reportBody := fmt.Sprintf("<@%d> Reported <@%d> in <#%d> For `%s`\nLast 100 messages from channel: <%s>", parsed.Msg.Author.ID, target, parsed.Msg.ChannelID, parsed.Args[1].Str(), logLink)

			_, err = common.BotSession.ChannelMessageSend(channelID, common.EscapeSpecialMentions(reportBody))
			if err != nil {
				return nil, err
			}

			// don't bother sending confirmation if it's in the same channel
			if channelID != parsed.Msg.ChannelID {
				return "User reported to the proper authorities", nil
			}
			return nil, nil
		},
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
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID, Default: 0},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "r", Name: "Regex", Type: dcmd.String},
			&dcmd.ArgDef{Switch: "ma", Default: time.Duration(0), Name: "Max age", Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Switch: "i", Name: "Regex case insensitive"},
		},
		ArgumentCombos: [][]int{[]int{0}, []int{0, 1}, []int{1, 0}},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, nil, config.CleanEnabled)
			if err != nil {
				return nil, err
			}

			userFilter := parsed.Args[1].Int64()

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
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Reason",
		Description:   "Add/Edit a modlog reason",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Message ID", Type: dcmd.Int},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionKickMembers, nil, true)
			if err != nil {
				return nil, err
			}

			if config.ActionChannel == "" {
				return "No mod log channel set up", nil
			}

			msg, err := common.BotSession.ChannelMessage(config.IntActionChannel(), parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			if msg.Author.ID != common.BotUser.ID {
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
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "Warn",
		Description:   "Warns a user, warnings are saved using the bot. Use -warnings to view them.",
		RequiredArgs:  2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Reason", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}
			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, config.WarnCmdRoles, config.WarnCommandsEnabled)
			if err != nil {
				return nil, err
			}

			err = WarnUser(config, parsed.GS.ID, parsed.CS.ID, parsed.Msg.Author, target, parsed.Args[1].Str())
			if err != nil {
				return nil, err
			}

			return GenericCmdResp(MAWarned, target, 0, false, true), nil
		},
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, config.WarnCmdRoles, true)
			if err != nil {
				return nil, err
			}

			userID := parsed.Args[0].Int64()

			var result []*WarningModel
			err = common.GORM.Where("user_id = ? AND guild_id = ?", userID, parsed.GS.ID).Order("id desc").Find(&result).Error
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
		},
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, config.WarnCmdRoles, config.WarnCommandsEnabled)
			if err != nil {
				return nil, err
			}

			rows := common.GORM.Model(WarningModel{}).Where("guild_id = ? AND id = ?", parsed.GS.ID, parsed.Args[0].Int()).Update(
				"message", fmt.Sprintf("%s (updated by %s#%s (%d))", parsed.Args[1].Str(), parsed.Msg.Author.Username, parsed.Msg.Author.Discriminator, parsed.Msg.Author.ID)).RowsAffected

			if rows < 1 {
				return "Failed updating, most likely couldn't find the warning", nil
			}

			return "👌", nil
		},
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, config.WarnCmdRoles, config.WarnCommandsEnabled)
			if err != nil {
				return nil, err
			}

			rows := common.GORM.Where("guild_id = ? AND id = ?", parsed.GS.ID, parsed.Args[0].Int()).Delete(WarningModel{}).RowsAffected
			if rows < 1 {
				return "Failed deleting, most likely couldn't find the warning", nil
			}

			return "👌", nil
		},
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
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {

			config, _, err := MBaseCmd(parsed, 0)
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageMessages, config.WarnCmdRoles, config.WarnCommandsEnabled)
			if err != nil {
				return nil, err
			}

			userID := parsed.Args[0].Int64()

			rows := common.GORM.Where("guild_id = ? AND user_id = ?", parsed.GS.ID, userID).Delete(WarningModel{}).RowsAffected
			return fmt.Sprintf("Deleted %d warnings.", rows), nil
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "GiveRole",
		Aliases:       []string{"grole", "arole", "addrole"},
		Description:   "Gives a role to the specified member, with optional expiry",

		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Role", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			&dcmd.ArgDef{Switch: "d", Default: time.Duration(0), Name: "Duration", Type: &commands.DurationArg{}},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageRoles, config.GiveRoleCmdRoles, config.GiveRoleCmdEnabled)
			if err != nil {
				return nil, err
			}

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			role := FindRole(parsed.GS, parsed.Args[1].Str())
			if role == nil {
				return "Couldn't find the specified role", nil
			}

			authorMember := commands.ContextMS(parsed.Context())
			parsed.GS.RLock()
			if !bot.IsMemberAboveRole(parsed.GS, authorMember, role) {
				parsed.GS.RUnlock()
				return "Can't give roles above you", nil
			}
			parsed.GS.RUnlock()

			dur := parsed.Switches["d"].Value.(time.Duration)

			// no point if the user has the role and is not updating the expiracy
			if common.ContainsInt64Slice(member.Roles, role.ID) && dur <= 0 {
				return "That user already has that role", nil
			}

			err = common.AddRoleDS(member, role.ID)
			if err != nil {
				return nil, err
			}

			// schedule the expirey
			if dur > 0 {
				err := scheduledevents2.ScheduleRemoveRole(parsed.Context(), parsed.GS.ID, target.ID, role.ID, time.Now().Add(dur))
				if err != nil {
					return nil, err
				}
			}

			action := MAGiveRole
			action.Prefix = "Gave the role " + role.Name + " to "
			if config.GiveRoleCmdModlog && config.IntActionChannel() != 0 {
				if dur > 0 {
					action.Footer = "Duration: " + common.HumanizeDuration(common.DurationPrecisionMinutes, dur)
				}
				CreateModlogEmbed(config.IntActionChannel(), parsed.Msg.Author, action, target, "", "")
			}

			return GenericCmdResp(action, target, dur, true, dur <= 0), nil
		},
	},
	&commands.YAGCommand{
		CustomEnabled: true,
		CmdCategory:   commands.CategoryModeration,
		Name:          "RemoveRole",
		Aliases:       []string{"rrole", "takerole", "trole"},
		Description:   "Removes the specified role from the target",

		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "User", Type: dcmd.UserID},
			&dcmd.ArgDef{Name: "Role", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			config, target, err := MBaseCmd(parsed, parsed.Args[0].Int64())
			if err != nil {
				return nil, err
			}

			_, err = MBaseCmdSecond(parsed, "", true, discordgo.PermissionManageRoles, config.GiveRoleCmdRoles, config.GiveRoleCmdEnabled)
			if err != nil {
				return nil, err
			}

			member, err := bot.GetMember(parsed.GS.ID, target.ID)
			if err != nil || member == nil {
				return "Member not found", err
			}

			role := FindRole(parsed.GS, parsed.Args[1].Str())
			if role == nil {
				return "Couldn't find the specified role", nil
			}

			authorMember := commands.ContextMS(parsed.Context())
			parsed.GS.RLock()
			if !bot.IsMemberAboveRole(parsed.GS, authorMember, role) {
				parsed.GS.RUnlock()
				return "Can't remove roles above you", nil
			}
			parsed.GS.RUnlock()

			err = common.RemoveRoleDS(member, role.ID)
			if err != nil {
				return nil, err
			}

			// cancel the event to remove the role
			scheduledevents2.CancelRemoveRole(parsed.Context(), parsed.GS.ID, parsed.Msg.Author.ID, role.ID)

			action := MARemoveRole
			action.Prefix = "Removed the role " + role.Name + " from "
			if config.GiveRoleCmdModlog && config.IntActionChannel() != 0 {
				CreateModlogEmbed(config.IntActionChannel(), parsed.Msg.Author, action, target, "", "")
			}

			return GenericCmdResp(action, target, 0, true, true), nil
		},
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

		// Can only bulk delete messages up to 2 weeks (but add 1 minute buffer account for time sync issues and other smallies)
		if now.Sub(msgs[i].ParsedCreated) > (time.Hour*24*14)-time.Minute {
			continue
		}

		// Check regex
		if compiledRegex != nil {
			if !compiledRegex.MatchString(msgs[i].Content) {
				continue
			}
		}

		// Check max age
		if maxAge != 0 && now.Sub(msgs[i].ParsedCreated) > maxAge {
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

func FindRole(gs *dstate.GuildState, roleS string) *discordgo.Role {
	parsedNumber, parseErr := strconv.ParseInt(roleS, 10, 64)

	gs.RLock()
	defer gs.RUnlock()
	if parseErr == nil {

		// was a number, try looking by id
		r := gs.RoleCopy(false, parsedNumber)
		if r != nil {
			return r
		}
	}

	// otherwise search by name
	for _, v := range gs.Guild.Roles {
		trimmed := strings.TrimSpace(v.Name)

		if strings.EqualFold(trimmed, roleS) {
			return v
		}
	}

	// couldn't find the role :(
	return nil
}
