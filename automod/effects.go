package automod

import (
	"context"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
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

	go bot.MessageDeleteQueue.DeleteMessages(ctxData.Message.ChannelID, ctxData.Message.ID)
	return nil
}

func (del *DeleteMessageEffect) MergeDuplicates(data []interface{}) interface{} {
	return nil // no user data
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

	return err
}

///////////////////////////////////////////////////////

type KickUserEffect struct{}

func (kick *KickUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (kick *KickUserEffect) DataType() interface{} {
	return nil
}

func (kick *KickUserEffect) Name() (name string) {
	return "Kick user"
}

func (kick *KickUserEffect) Description() (description string) {
	return "Kicks the user"
}

func (kick *KickUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (kick *KickUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	var cID int64

	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	err := moderation.KickUser(nil, ctxData.GS.ID, cID, common.BotUser, "Automoderator:\n"+ctxData.ConstructReason(true), ctxData.MS.DGoUser())
	return err
}

func (kick *KickUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type BanUserEffect struct{}

func (ban *BanUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (ban *BanUserEffect) DataType() interface{} {
	return nil
}

func (ban *BanUserEffect) Name() (name string) {
	return "Ban user"
}

func (ban *BanUserEffect) Description() (description string) {
	return "Bans the user"
}

func (ban *BanUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (ban *BanUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	var cID int64
	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	err := moderation.BanUser(nil, ctxData.GS.ID, cID, common.BotUser, "Automoderator:\n"+ctxData.ConstructReason(true), ctxData.MS.DGoUser())
	return err
}

func (ban *BanUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type MuteUserEffect struct{}

type MuteUserEffectData struct {
	Duration int
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
			Name:    "Duration",
			Key:     "Duration",
			Min:     1,
			Max:     10080, // 7 days
			Kind:    SettingTypeInt,
			Default: 10,
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

	err := moderation.MuteUnmuteUser(nil, true, ctxData.GS.ID, cID, common.BotUser, "Automoderator:\n"+ctxData.ConstructReason(true), ctxData.MS, settingsCast.Duration)
	return err
}

func (mute *MuteUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type WarnUserEffect struct{}

func (warn *WarnUserEffect) Kind() RulePartType {
	return RulePartEffect
}

func (warn *WarnUserEffect) DataType() interface{} {
	return &MuteUserEffectData{}
}

func (warn *WarnUserEffect) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (warn *WarnUserEffect) Name() (name string) {
	return "Warn user"
}

func (warn *WarnUserEffect) Description() (description string) {
	return "Warns the user"
}

func (warn *WarnUserEffect) Apply(ctxData *TriggeredRuleData, settings interface{}) error {
	// settingsCast := settings.(*MuteUserEffectData)

	var cID int64
	if ctxData.CS != nil {
		cID = ctxData.CS.ID
	}

	err := moderation.WarnUser(nil, ctxData.GS.ID, cID, common.BotUser, ctxData.MS.DGoUser(), "Automoderator:\n"+ctxData.ConstructReason(true))
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

	err := common.BotSession.GuildMemberNickname(ctxData.GS.ID, ctxData.MS.ID, settingsCast.NewName)
	return err
}

func (sn *SetNicknameEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}
