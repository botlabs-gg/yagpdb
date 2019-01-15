package automod

import (
	"context"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"time"
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
		bot.MessageDeleteQueue.DeleteMessages(cID, messages...)
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
	return "Delete mutliple messages"
}

func (del *DeleteMessagesEffect) Description() (description string) {
	return "Deletes a certain number of the users last messages in this channel"
}

func (del *DeleteMessagesEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Number of messages",
			Key:     "NumMessages",
			Kind:    SettingTypeInt,
			Min:     1,
			Max:     100,
			Default: 3,
		},
		&SettingDef{
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

	ctxData.GS.RLock()
	defer ctxData.GS.RUnlock()

	var channel *dstate.ChannelState
	if ctxData.CS != nil {
		channel = ctxData.CS
	} else {

		// no channel in context, attempt to find the last channel the user spoke in
		var lastMessage *dstate.MessageState

		for _, c := range ctxData.GS.Channels {
			for i := len(c.Messages) - 1; i >= 0; i-- {
				cMsg := c.Messages[i]

				if settingsCast.TimeLimit > 0 && timeLimit.After(cMsg.ParsedCreated) {
					break
				}

				if lastMessage != nil && lastMessage.ParsedCreated.After(cMsg.ParsedCreated) {
					break
				}

				if cMsg.Message.Author.ID == ctxData.MS.ID {
					channel = c
					lastMessage = cMsg
					break
				}
			}
		}
	}

	if channel == nil {
		return nil
	}

	messages := make([]int64, 0, 100)

	for i := len(channel.Messages) - 1; i >= 0; i-- {
		cMsg := channel.Messages[i]

		if settingsCast.TimeLimit > 0 && timeLimit.After(cMsg.ParsedCreated) {
			break
		}

		if cMsg.Message.Author.ID != ctxData.MS.ID {
			continue
		}

		messages = append(messages, cMsg.Message.ID)
		if len(messages) >= 100 || len(messages) >= settingsCast.NumMessages {
			break
		}
	}

	if len(messages) < 0 {
		return nil
	}

	go func(cID int64, messages []int64) {
		// deleting messages too fast can sometimes make them still show in the discord client even after deleted
		time.Sleep(500 * time.Millisecond)
		bot.MessageDeleteQueue.DeleteMessages(cID, messages...)
	}(channel.ID, messages)

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
	return "Adds a violation (use with violation tirggers)"
}

func (vio *AddViolationEffect) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Name",
			Key:     "Name",
			Kind:    SettingTypeString,
			Min:     1,
			Max:     50,
			Default: "violation name",
		},
	}
}

func (vio *AddViolationEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*AddViolationEffectData)
	violation := &models.AutomodViolation{
		GuildID: ctxData.GS.ID,
		UserID:  ctxData.MS.ID,
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

	logrus.Debug("Added violation to ", settingsCast.Name)

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
		&SettingDef{
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

	var cID int64

	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.KickUser(nil, ctxData.GS.ID, cID, common.BotUser, reason, ctxData.MS.DGoUser())
	return err
}

func (kick *KickUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type BanUserEffect struct{}

type BanUserEffectData struct {
	Duration     int
	CustomReason string `valid:",0,150,trimspace"`
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
		&SettingDef{
			Name:    "Duration (minutes, 0 for permanent)",
			Key:     "Duration",
			Kind:    SettingTypeInt,
			Default: 0,
		},
		&SettingDef{
			Name: "Custom message (empty for default)",
			Key:  "CustomReason",
			Min:  0,
			Max:  150,
			Kind: SettingTypeString,
		},
	}
}

func (ban *BanUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	settingsCast := settings.(*BanUserEffectData)

	var cID int64
	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	duration := time.Duration(settingsCast.Duration) * time.Minute
	err := moderation.BanUserWithDuration(nil, ctxData.GS.ID, cID, common.BotUser, reason, ctxData.MS.DGoUser(), duration)
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
		&SettingDef{
			Name:    "Duration (minutes)",
			Key:     "Duration",
			Min:     1,
			Max:     10080, // 7 days
			Kind:    SettingTypeInt,
			Default: 10,
		},
		&SettingDef{
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

	var cID int64
	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.MuteUnmuteUser(nil, true, ctxData.GS.ID, cID, common.BotUser, reason, ctxData.MS, settingsCast.Duration)
	return err
}

func (mute *MuteUserEffect) MergeDuplicates(data []interface{}) interface{} {
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
		&SettingDef{
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

	var cID int64
	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.WarnUser(nil, ctxData.GS.ID, cID, common.BotUser, ctxData.MS.DGoUser(), reason)
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
		&SettingDef{
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

	curNick := ""
	ctxData.GS.RLock()
	curNick = ctxData.MS.Nick
	ctxData.GS.RUnlock()

	if curNick == settingsCast.NewName {
		// Avoid infinite recursion
		return nil
	}

	logrus.WithField("guild", ctxData.GS.ID).Info("automod: set nickname: ", settingsCast.NewName)
	err := common.BotSession.GuildMemberNickname(ctxData.GS.ID, ctxData.MS.ID, settingsCast.NewName)
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
		&SettingDef{
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
	_, err := models.AutomodViolations(qm.Where("guild_id = ? AND user_id = ? AND name = ?", ctxData.GS.ID, ctxData.MS.ID, settingsCast.Name)).DeleteAll(context.Background(), common.PQ)
	return err
}
