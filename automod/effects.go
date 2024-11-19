package automod

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/automod/models"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	schEventsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type Effect interface {
	Apply(ctxData *TriggeredRuleData, settings interface{}) error
}

///////////////////////////////////////////////////////

type DeleteMessageEffect struct{}

func (del *DeleteMessageEffect) Kind() RulePartType {
	return RulePartEffect
}

func (del *DeleteMessageEffect) DataType() interface{} {
	return nil
}

func (del *DeleteMessageEffect) Name() (name string) {
	return "Delete Message"
}

func (del *DeleteMessageEffect) Description() (description string) {
	return "Deletes the message"
}

func (del *DeleteMessageEffect) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (del *DeleteMessageEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	if ctxData.Message == nil {
		return nil // no message to delete
	}

	go func(cID int64, messages []int64) {
		// deleting messages too fast can sometimes make them still show in the discord client even after deleted
		time.Sleep(500 * time.Millisecond)
		bot.MessageDeleteQueue.DeleteMessages(ctxData.GS.ID, cID, messages...)
	}(ctxData.Message.ChannelID, []int64{ctxData.Message.ID})

	return nil
}

func (del *DeleteMessageEffect) MergeDuplicates(data []interface{}) interface{} {
	return nil // no user data
}

///////////////////////////////////////////////////////

type DeleteMessagesEffectData struct {
	NumMessages int
	TimeLimit   int
}

type DeleteMessagesEffect struct{}

func (del *DeleteMessagesEffect) Kind() RulePartType {
	return RulePartEffect
}

func (del *DeleteMessagesEffect) DataType() interface{} {
	return &DeleteMessagesEffectData{}
}

func (del *DeleteMessagesEffect) Name() (name string) {
	return "Delete multiple messages"
}

func (del *DeleteMessagesEffect) Description() (description string) {
	return "Deletes a certain number of the users last messages in this channel"
}

func (del *DeleteMessagesEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Number of messages",
			Key:     "NumMessages",
			Kind:    SettingTypeInt,
			Min:     1,
			Max:     100,
			Default: 3,
		},
		{
			Name:    "Max age (seconds)",
			Key:     "TimeLimit",
			Kind:    SettingTypeInt,
			Min:     1,
			Max:     1000000,
			Default: 15,
		},
	}
}

func (del *DeleteMessagesEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {

	settingsCast := settings.(*DeleteMessagesEffectData)
	timeLimit := time.Now().Add(-time.Second * time.Duration(settingsCast.TimeLimit))

	var channel *dstate.ChannelState
	if ctxData.CS != nil {
		channel = ctxData.CS
	} else {
		// do nothing for now
		return nil
	}

	if channel == nil {
		return nil
	}

	messages := bot.State.GetMessages(ctxData.GS.ID, ctxData.CS.ID, &dstate.MessagesQuery{
		Limit: 1000,
	})

	deleteMessages := make([]int64, 0)

	for _, cMsg := range messages {
		if settingsCast.TimeLimit > 0 && timeLimit.After(cMsg.ParsedCreatedAt) {
			break
		}

		if cMsg.Author.ID != ctxData.MS.User.ID {
			continue
		}

		deleteMessages = append(deleteMessages, cMsg.ID)
		if len(deleteMessages) >= 100 || len(deleteMessages) >= settingsCast.NumMessages {
			break
		}
	}

	go func(cs *dstate.ChannelState, messages []int64) {
		// deleting messages too fast can sometimes make them still show in the discord client even after deleted
		time.Sleep(500 * time.Millisecond)
		bot.MessageDeleteQueue.DeleteMessages(cs.GuildID, cs.ID, messages...)
	}(channel, deleteMessages)

	return nil
}

func (del *DeleteMessagesEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type AddViolationEffect struct{}

type AddViolationEffectData struct {
	Name string `valid:",1,100,trimspace"`
}

func (vio *AddViolationEffect) Kind() RulePartType {
	return RulePartEffect
}

func (vio *AddViolationEffect) DataType() interface{} {
	return &AddViolationEffectData{}
}

func (vio *AddViolationEffect) Name() (name string) {
	return "+Violation"
}

func (vio *AddViolationEffect) Description() (description string) {
	return "Adds a violation (use with violation triggers)"
}

func (vio *AddViolationEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:        "Name",
			Key:         "Name",
			Kind:        SettingTypeString,
			Min:         1,
			Max:         50,
			Placeholder: "Enter name for the violation",
		},
	}
}

func (vio *AddViolationEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*AddViolationEffectData)
	violation := &models.AutomodViolation{
		GuildID: ctxData.GS.ID,
		UserID:  ctxData.MS.User.ID,
		RuleID:  null.Int64From(ctxData.CurrentRule.Model.ID),
		Name:    settingsCast.Name,
	}

	err := violation.InsertG(context.Background(), boil.Infer())
	if err != nil {
		return err
	}

	newData := ctxData.Clone()
	newData.PreviousReasons = append(newData.PreviousReasons, ctxData.ConstructReason(false))
	newData.RecursionCounter++
	go ctxData.Plugin.checkViolationTriggers(newData, settingsCast.Name)

	logger.Debug("Added violation to ", settingsCast.Name)

	return err
}

///////////////////////////////////////////////////////

type KickUserEffect struct{}

type KickUserEffectData struct {
	CustomReason string `valid:",0,150,trimspace"`
}

func (kick *KickUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (kick *KickUserEffect) DataType() interface{} {
	return &KickUserEffectData{}
}

func (kick *KickUserEffect) Name() (name string) {
	return "Kick user"
}

func (kick *KickUserEffect) Description() (description string) {
	return "Kicks the user"
}

func (kick *KickUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
	}
}

func (kick *KickUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*KickUserEffectData)

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.KickUser(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, &ctxData.MS.User, -1, false)
	return err
}

func (kick *KickUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type BanUserEffect struct{}

type BanUserEffectData struct {
	Duration          int
	CustomReason      string `valid:",0,150,trimspace"`
	MessageDeleteDays int
}

func (ban *BanUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (ban *BanUserEffect) DataType() interface{} {
	return &BanUserEffectData{}
}

func (ban *BanUserEffect) Name() (name string) {
	return "Ban user"
}

func (ban *BanUserEffect) Description() (description string) {
	return "Bans the user"
}

func (ban *BanUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration (minutes, 0 for permanent)",
			Key:     "Duration",
			Kind:    SettingTypeInt,
			Default: 0,
		},
		{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
		{
			Name:    "Number of days of messages to delete (0 to 7)",
			Key:     "MessageDeleteDays",
			Kind:    SettingTypeInt,
			Min:     0,
			Max:     7,
			Default: 1,
		},
	}
}

func (ban *BanUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*BanUserEffectData)

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	duration := time.Duration(settingsCast.Duration) * time.Minute
	err := moderation.BanUserWithDuration(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, &ctxData.MS.User, duration, settingsCast.MessageDeleteDays, false)
	return err
}

func (ban *BanUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type MuteUserEffect struct{}

type MuteUserEffectData struct {
	Duration     int
	CustomReason string `valid:",0,150,trimspace"`
}

func (mute *MuteUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (mute *MuteUserEffect) DataType() interface{} {
	return &MuteUserEffectData{}
}

func (mute *MuteUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration (minutes, 0 for permanent)",
			Key:     "Duration",
			Min:     0,
			Kind:    SettingTypeInt,
			Default: 10,
		},
		{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
	}
}

func (mute *MuteUserEffect) Name() (name string) {
	return "Mute user"
}

func (mute *MuteUserEffect) Description() (description string) {
	return "Mutes the user"
}

func (mute *MuteUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*MuteUserEffectData)

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.MuteUnmuteUser(nil, true, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, ctxData.MS, settingsCast.Duration, false)
	return err
}

func (mute *MuteUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type TimeoutUserEffect struct{}

type TimeoutUserEffectData struct {
	Duration     int    `valid:",0,40320,trimspace"`
	CustomReason string `valid:",0,150,trimspace"`
}

func (timeout *TimeoutUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (timeout *TimeoutUserEffect) DataType() interface{} {
	return &TimeoutUserEffectData{}
}

func (timeout *TimeoutUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration (minutes)",
			Key:     "Duration",
			Min:     int(moderation.MinTimeOutDuration.Minutes()),
			Max:     int(moderation.MaxTimeOutDuration.Minutes()),
			Kind:    SettingTypeInt,
			Default: int(moderation.DefaultTimeoutDuration.Minutes()),
		},
		{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
	}
}

func (timeout *TimeoutUserEffect) Name() (name string) {
	return "Timeout user"
}

func (timeout *TimeoutUserEffect) Description() (description string) {
	return "Timeout the user"
}

func (timeout *TimeoutUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	// if a user is timed out, do not apply the effect again.
	member := ctxData.MS.Member
	if member.CommunicationDisabledUntil != nil && member.CommunicationDisabledUntil.After(time.Now()) {
		return nil
	}

	settingsCast := settings.(*TimeoutUserEffectData)

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	duration := time.Duration(settingsCast.Duration) * time.Minute
	err := moderation.TimeoutUser(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, &ctxData.MS.User, duration, false)
	return err
}

func (timeout *TimeoutUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type WarnUserEffect struct{}

type WarnUserEffectData struct {
	CustomReason string `valid:",0,150,trimspace"`
}

func (warn *WarnUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (warn *WarnUserEffect) DataType() interface{} {
	return &WarnUserEffectData{}
}

func (warn *WarnUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
	}
}

func (warn *WarnUserEffect) Name() (name string) {
	return "Warn user"
}

func (warn *WarnUserEffect) Description() (description string) {
	return "Warns the user, with an optional custom warning message"
}

func (warn *WarnUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*WarnUserEffectData)

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.WarnUser(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, &ctxData.MS.User, reason, false)
	return err
}

func (warn *WarnUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return nil // no user data
}

/////////////////////////////////////////////////////////////

type SetNicknameEffect struct{}

type SetNicknameEffectData struct {
	NewName string `valid:",0,32,trimspace"`
}

func (sn *SetNicknameEffect) Kind() RulePartType {
	return RulePartEffect
}

func (sn *SetNicknameEffect) DataType() interface{} {
	return &SetNicknameEffectData{}
}

func (sn *SetNicknameEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "New Nickname (empty for removal)",
			Key:  "NewName",
			Min:  0,
			Max:  32,
			Kind: SettingTypeString,
		},
	}
}

func (sn *SetNicknameEffect) Name() (name string) {
	return "Set Nickname"
}

func (sn *SetNicknameEffect) Description() (description string) {
	return "Sets the nickname of the user"
}

func (sn *SetNicknameEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*SetNicknameEffectData)

	if ctxData.MS.Member.Nick == settingsCast.NewName {
		// Avoid infinite recursion
		return nil
	}

	logger.WithField("guild", ctxData.GS.ID).Info("set nickname: ", settingsCast.NewName)
	err := common.BotSession.GuildMemberNickname(ctxData.GS.ID, ctxData.MS.User.ID, settingsCast.NewName)
	return err
}

func (sn *SetNicknameEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

/////////////////////////////////////////////////////////////

type ResetViolationsEffect struct{}

type ResetViolationsEffectData struct {
	Name string `valid:",0,50,trimspace"`
}

func (rv *ResetViolationsEffect) Kind() RulePartType {
	return RulePartEffect
}

func (rv *ResetViolationsEffect) DataType() interface{} {
	return &ResetViolationsEffectData{}
}

func (rv *ResetViolationsEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Name",
			Key:     "Name",
			Default: "name",
			Min:     0,
			Max:     50,
			Kind:    SettingTypeString,
		},
	}
}

func (rv *ResetViolationsEffect) Name() (name string) {
	return "Reset violations"
}

func (rv *ResetViolationsEffect) Description() (description string) {
	return "Resets the violations of a user"
}

func (rv *ResetViolationsEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*ResetViolationsEffectData)
	_, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ? AND name = ?", ctxData.GS.ID, ctxData.MS.User.ID, settingsCast.Name)).DeleteAll(context.Background(), common.PQ)
	return err
}

/////////////////////////////////////////////////////////////

type GiveRoleEffect struct{}

type GiveRoleEffectData struct {
	Duration int `valid:",0,604800,trimspace"`
	Role     int64
}

func (gr *GiveRoleEffect) Kind() RulePartType {
	return RulePartEffect
}

func (gf *GiveRoleEffect) DataType() interface{} {
	return &GiveRoleEffectData{}
}

func (gf *GiveRoleEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration in seconds, 0 for permanent",
			Key:     "Duration",
			Default: 0,
			Min:     0,
			Max:     604800,
			Kind:    SettingTypeInt,
		},
		{
			Name: "Role",
			Key:  "Role",
			Kind: SettingTypeRole,
		},
	}
}

func (gf *GiveRoleEffect) Name() (name string) {
	return "Give role"
}

func (gf *GiveRoleEffect) Description() (description string) {
	return "Gives the specified role to the user, optionally with a duration after which the role is removed from the user."
}

func (gf *GiveRoleEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*GiveRoleEffectData)

	err := common.AddRoleDS(ctxData.MS, settingsCast.Role)
	if err != nil {
		if code, _ := common.DiscordError(err); code != 0 {
			return nil // discord responded with a proper error, we know that it didn't break
		}

		// discord was not the cause of the error, in some cases even if the gateway times out the action is performed so just in case, scehdule the role removal
	}

	if settingsCast.Duration > 0 {
		err := scheduledevents2.ScheduleRemoveRole(context.Background(), ctxData.GS.ID, ctxData.MS.User.ID, settingsCast.Role, time.Now().Add(time.Second*time.Duration(settingsCast.Duration)))
		if err != nil {
			return err
		}
	}

	return nil
}

//////////////////////////////////////////////////////////////

type RemoveRoleEffect struct{}

type RemoveRoleEffectData struct {
	Duration int `valid:",0,604800,trimspace"`
	Role     int64
}

func (rr *RemoveRoleEffect) Kind() RulePartType {
	return RulePartEffect
}

func (rf *RemoveRoleEffect) DataType() interface{} {
	return &RemoveRoleEffectData{}
}

func (rf *RemoveRoleEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration in seconds, 0 for permanent",
			Key:     "Duration",
			Default: 0,
			Min:     0,
			Max:     604800,
			Kind:    SettingTypeInt,
		},
		{
			Name: "Role",
			Key:  "Role",
			Kind: SettingTypeRole,
		},
	}
}

func (rf *RemoveRoleEffect) Name() (name string) {
	return "Remove role"
}

func (rf *RemoveRoleEffect) Description() (description string) {
	return "Removes the specified role from the user, optionally with a duration after which the role is added back to the user."
}

func (rf *RemoveRoleEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*RemoveRoleEffectData)

	if !common.ContainsInt64Slice(ctxData.MS.Member.Roles, settingsCast.Role) {
		return nil
	}

	err := common.RemoveRoleDS(ctxData.MS, settingsCast.Role)
	if err != nil {
		if code, _ := common.DiscordError(err); code != 0 {
			return nil // discord responded with a proper error, we know that it didn't break
		}

		// discord was not the cause of the error, in some cases even if the gateway times out the action is performed so just in case, scehdule the role add
	}

	if settingsCast.Duration > 0 {
		err := scheduledevents2.ScheduleAddRole(context.Background(), ctxData.GS.ID, ctxData.MS.User.ID, settingsCast.Role, time.Now().Add(time.Second*time.Duration(settingsCast.Duration)))
		if err != nil {
			return err
		}
	}

	return nil
}

/////////////////////////////////////////////////////////////

type SendChannelMessageEffectData struct {
	CustomReason string `valid:",0,280,trimspace"`
	Duration     int    `valid:",0,3600,trimspace"`
	PingUser     bool
	LogChannel   int64
}

type SendChannelMessageEffect struct{}

func (send *SendChannelMessageEffect) Kind() RulePartType {
	return RulePartEffect
}

func (send *SendChannelMessageEffect) DataType() interface{} {
	return &SendChannelMessageEffectData{}
}

func (send *SendChannelMessageEffect) Name() (name string) {
	return "Send Message"
}

func (send *SendChannelMessageEffect) Description() (description string) {
	return "Sends the message on the channel the rule was triggered"
}

func (send *SendChannelMessageEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Custom message",
			Key:  "CustomReason",
			Min:  0,
			Max:  280,
			Kind: SettingTypeString,
		},
		{
			Name:    "Delete sent message after x seconds (0 for non-deletion)",
			Key:     "Duration",
			Kind:    SettingTypeInt,
			Default: 0,
			Min:     0,
			Max:     3600,
		},
		{
			Name:    "Ping user committing the infraction",
			Key:     "PingUser",
			Kind:    SettingTypeBool,
			Default: false,
		},
		{
			Name:    "Channel to send message in (Leave None to send message in same channel)",
			Key:     "LogChannel",
			Kind:    SettingTypeChannel,
			Default: nil,
		},
	}
}

func (send *SendChannelMessageEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	// Ignore bots
	if ctxData.MS.User.Bot {
		return nil
	}

	settingsCast := settings.(*SendChannelMessageEffectData)

	// If we dont have any channel data, we can't send a message
	if ctxData.CS == nil && settingsCast.LogChannel == 0 {
		return nil
	}

	msgSend := &discordgo.MessageSend{}

	if settingsCast.PingUser {
		msgSend.Content = "<@" + discordgo.StrID(ctxData.MS.User.ID) + ">\n"
		msgSend.AllowedMentions = discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		}
	}

	msgSend.Content += "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		msgSend.Content += settingsCast.CustomReason
	} else {
		msgSend.Content += ctxData.ConstructReason(true)
	}

	var logChannel int64
	if settingsCast.LogChannel != 0 {
		logChannel = settingsCast.LogChannel
	} else {
		logChannel = ctxData.CS.ID
	}

	message, err := common.BotSession.ChannelMessageSendComplex(logChannel, msgSend)
	if err != nil {
		logger.WithError(err).Error("Failed to send message for AutomodV2")
		return err
	}
	if settingsCast.Duration > 0 && message != nil {
		templates.MaybeScheduledDeleteMessage(ctxData.GS.ID, logChannel, message.ID, settingsCast.Duration, "")
	}
	return nil
}

func (send *SendChannelMessageEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no user data
}

/////////////////////////////////////////////////////////////

type SendModeratorAlertMessageData struct {
	CustomMessage string `valid:",0,280,trimspace"`
	LogChannel   int64
}

type SendModeratorAlertMessageEffect struct{}

func (send *SendModeratorAlertMessageEffect) Kind() RulePartType {
	return RulePartEffect
}

func (send *SendModeratorAlertMessageEffect) DataType() interface{} {
	return &SendModeratorAlertMessageData{}
}

func (send *SendModeratorAlertMessageEffect) Name() (name string) {
	return "Send Alert"
}

func (send *SendModeratorAlertMessageEffect) Description() (description string) {
	return "Sends an embed to the specified channel with info about the triggered rule"
}

func (send *SendModeratorAlertMessageEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Custom message",
			Key:  "CustomMessage",
			Min:  0,
			Max:  280,
			Kind: SettingTypeString,
		},
		{
			Name:    "Channel to send alert embed in",
			Key:     "LogChannel",
			Kind:    SettingTypeChannel,
			Default: nil,
		},
	}
}

func (send *SendModeratorAlertMessageEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	// Ignore bots
	if ctxData.MS.User.Bot {
		return nil
	}

	settingsCast := settings.(*SendModeratorAlertMessageData)

	// If we dont have any channel data, we can't send a message
	if ctxData.CS == nil && settingsCast.LogChannel == 0 {
		return nil
	}

	msgSend := &discordgo.MessageSend{}

	if ctxData.CS != nil {
		msgSend.Content = fmt.Sprintf("Automoderator alert triggered in <#%d>:\n", ctxData.CS.ID)
	} else {
		msgSend.Content = "Automoderator alert:\n"
	}

	if settingsCast.CustomMessage != "" {
		msgSend.Content += settingsCast.CustomMessage
		msgSend.AllowedMentions = discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeUsers},
		}
	}

	msgEmbed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("%s (ID: %d)", ctxData.MS.User.Username, ctxData.MS.User.ID),
			IconURL: ctxData.MS.User.AvatarURL("64"),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: ctxData.ConstructReason(false),
		},
	}

	msgEmbed.Fields = []*discordgo.MessageEmbedField{{
		Name: "____",
		Value: ctxData.MS.User.Mention(),
	}}

	if ctxData.Message != nil {
		msgEmbed.Description = ctxData.Message.Content
		existingValue := msgEmbed.Fields[0].Value
		msgEmbed.Fields[0].Value = fmt.Sprintf("[Jump to Message](%s) • %s", ctxData.Message.Link(), existingValue)
	}

	msgSend.Embeds = []*discordgo.MessageEmbed{msgEmbed}

	var logChannel int64
	if settingsCast.LogChannel != 0 {
		logChannel = settingsCast.LogChannel
	} else {
		logChannel = ctxData.CS.ID
	}

	_, err := common.BotSession.ChannelMessageSendComplex(logChannel, msgSend)
	if err != nil {
		logger.WithError(err).Error("Failed to send mod alert for AutomodV2")
		return err
	}

	return nil
}

func (send *SendModeratorAlertMessageEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no user data
}

/////////////////////////////////////////////////////////////

type EnableChannelSlowmodeEffect struct {
	lastTimes map[int64]bool
	mu        sync.Mutex
}

type EnableChannelSlowmodeEffectData struct {
	Duration  int `valid:",0,604800,trimspace"`
	Ratelimit int `valid:",0,21600,trimspace"`
}

func (slow *EnableChannelSlowmodeEffect) Kind() RulePartType {
	return RulePartEffect
}

func (slow *EnableChannelSlowmodeEffect) DataType() interface{} {
	return &EnableChannelSlowmodeEffectData{}
}

func (slow *EnableChannelSlowmodeEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Duration in seconds, 0 for permanent",
			Key:     "Duration",
			Default: 0,
			Min:     0,
			Max:     604800,
			Kind:    SettingTypeInt,
		},
		{
			Name:    "Ratelimit in seconds between messages per user",
			Key:     "Ratelimit",
			Default: 0,
			Min:     0,
			Max:     21600,
			Kind:    SettingTypeInt,
		},
	}
}

func (slow *EnableChannelSlowmodeEffect) Name() (name string) {
	return "Enable Channel slowmode"
}

func (slow *EnableChannelSlowmodeEffect) Description() (description string) {
	return "Enables discord's builtin slowmode in the channel for the specified duration, or forever."
}

func (slow *EnableChannelSlowmodeEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	if ctxData.CS == nil {
		return nil
	}

	if slow.checkSetCooldown(ctxData.CS.ID) {
		return nil
	}

	s := settings.(*EnableChannelSlowmodeEffectData)

	rl := s.Ratelimit
	edit := &discordgo.ChannelEdit{
		RateLimitPerUser: &rl,
	}

	_, err := common.BotSession.ChannelEditComplex(ctxData.CS.ID, edit)
	if err != nil {
		return err
	}

	if s.Duration < 1 {
		return nil
	}

	// remove existing role removal events for this channel
	_, err = schEventsModels.ScheduledEvents(
		qm.Where("event_name='amod2_reset_channel_ratelimit'"),
		qm.Where("guild_id = ?", ctxData.GS.ID),
		qm.Where("(data->>'channel_id')::bigint = ?", ctxData.CS.ID),
		qm.Where("processed = false")).DeleteAll(context.Background(), common.PQ)

	if err != nil {
		return err
	}

	// add the scheduled event for it
	err = scheduledevents2.ScheduleEvent("amod2_reset_channel_ratelimit", ctxData.GS.ID, time.Now().Add(time.Second*time.Duration(s.Duration)), &ResetChannelRatelimitData{
		ChannelID: ctxData.CS.ID,
	})

	if err != nil {
		return err
	}

	return nil
}

func (slow *EnableChannelSlowmodeEffect) checkSetCooldown(channelID int64) bool {
	slow.mu.Lock()
	defer slow.mu.Unlock()

	if slow.lastTimes == nil {
		slow.lastTimes = make(map[int64]bool)
		return false
	}

	if v, ok := slow.lastTimes[channelID]; ok && v {
		return true
	}

	slow.lastTimes[channelID] = true
	time.AfterFunc(time.Second*10, func() {
		slow.mu.Lock()
		defer slow.mu.Unlock()

		delete(slow.lastTimes, channelID)
	})

	return false
}
