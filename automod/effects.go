package automod

import (
	"context"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

type Effect interface {
	Apply(ctxData *TriggeredRuleData, settings interface{}) error
}

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

	err := common.BotSession.ChannelMessageDelete(ctxData.Message.ChannelID, ctxData.Message.ID)
	return err
}

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
	return "Adds a violation"
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
		RuleID:  null.Int64From(ctxData.Rule.Model.ID),
		Name:    settingsCast.Name,
	}

	err := violation.InsertG(context.Background(), boil.Infer())
	if err != nil {
		return err
	}

	go ctxData.Plugin.checkViolationTriggers(ctxData.GS, ctxData.MS, settingsCast.Name)

	return err
}

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

	err := moderation.KickUser(nil, ctxData.GS.ID, cID, common.BotUser, "Automoderator: TODO", ctxData.MS.DGoUser())
	return err
}

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

	err := moderation.BanUser(nil, ctxData.GS.ID, cID, common.BotUser, "Automoderator: TODO", ctxData.MS.DGoUser())
	return err
}

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

	err := moderation.MuteUnmuteUser(nil, true, ctxData.GS.ID, cID, common.BotUser, "Automoderator: TODO", ctxData.MS, settingsCast.Duration)
	return err
}
