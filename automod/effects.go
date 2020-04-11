package automod

import (
	"context"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
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

				if cMsg.Author.ID == ctxData.MS.ID {
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

		if cMsg.Author.ID != ctxData.MS.ID {
			continue
		}

		messages = append(messages, cMsg.ID)
		if len(messages) >= 100 || len(messages) >= settingsCast.NumMessages {
			break
		}
	}

	if len(messages) < 0 {
		return nil
	}

	go func(cs *dstate.ChannelState, messages []int64) {
		// deleting messages too fast can sometimes make them still show in the discord client even after deleted
		time.Sleep(500 * time.Millisecond)
		bot.MessageDeleteQueue.DeleteMessages(cs.Guild.ID, cs.ID, messages...)
	}(channel, messages)

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

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.KickUser(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, ctxData.MS.DGoUser())
	return err
}

func (kick *KickUserEffect) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}

///////////////////////////////////////////////////////

type BanUserEffect struct{}

type BanUserEffectData struct {
	Duration     	  int
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
		&SettingDef{
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
	err := moderation.BanUserWithDuration(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, ctxData.MS.DGoUser(), duration, settingsCast.MessageDeleteDays)
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
			Name:    "Duration (minutes, 0 for permanent)",
			Key:     "Duration",
			Min:  	 0,
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

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.MuteUnmuteUser(nil, true, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, reason, ctxData.MS, settingsCast.Duration)
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

	reason := "Automoderator:\n"
	if settingsCast.CustomReason != "" {
		reason += settingsCast.CustomReason
	} else {
		reason += ctxData.ConstructReason(true)
	}

	err := moderation.WarnUser(nil, ctxData.GS.ID, ctxData.CS, ctxData.Message, common.BotUser, ctxData.MS.DGoUser(), reason)
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

	logger.WithField("guild", ctxData.GS.ID).Info("set nickname: ", settingsCast.NewName)
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
		&SettingDef{
			Name:    "Duration in seconds, 0 for permanent",
			Key:     "Duration",
			Default: 0,
			Min:     0,
			Max:     604800,
			Kind:    SettingTypeInt,
		},
		&SettingDef{
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
		err := scheduledevents2.ScheduleRemoveRole(context.Background(), ctxData.GS.ID, ctxData.MS.ID, settingsCast.Role, time.Now().Add(time.Second*time.Duration(settingsCast.Duration)))
		if err != nil {
			return err
		}
	}

	return nil
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
		&SettingDef{
			Name:    "Duration in seconds, 0 for permanent",
			Key:     "Duration",
			Default: 0,
			Min:     0,
			Max:     604800,
			Kind:    SettingTypeInt,
		},
		&SettingDef{
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
