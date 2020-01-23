package automod

import (
	"time"

	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

type Condition interface {
	RulePart

	// IsMet is called to check wether this condition is met or not
	IsMet(data *TriggeredRuleData, parsedSettings interface{}) (bool, error)
}

/////////////////////////////////////////////////////////////////

type MemberRolesConditionData struct {
	Roles []int64
}

var _ Condition = (*MemberRolesCondition)(nil)

type MemberRolesCondition struct {
	Blacklist bool // if true, then blacklist mode, otherwise whitelist mode
}

func (mrc *MemberRolesCondition) Kind() RulePartType {
	return RulePartCondition
}

func (mrc *MemberRolesCondition) DataType() interface{} {
	return &MemberRolesConditionData{}
}

func (mrc *MemberRolesCondition) Name() string {
	if mrc.Blacklist {
		return "Ignore roles"
	}

	return "Require roles"
}

func (mrc *MemberRolesCondition) Description() string {
	if mrc.Blacklist {
		return "Ignore users with atleast one of these roles from this rule"
	}

	return "Require atleast one of these roles on the user"
}

func (mrc *MemberRolesCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "Roles",
			Key:  "Roles",
			Kind: SettingTypeMultiRole,
		},
	}
}

func (mrc *MemberRolesCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*MemberRolesConditionData)
	for _, r := range settingsCast.Roles {
		if common.ContainsInt64Slice(data.MS.Roles, r) {
			if mrc.Blacklist {
				// Had a blacklist role, this condition is not met
				return false, nil
			} else {
				// Had a whitelist role, this condition is met
				return true, nil
			}
		}
	}

	if mrc.Blacklist {
		// Did not have a blacklist role
		return true, nil
	}

	// Did not have a whitelist role
	return false, nil
}

func (mrc *MemberRolesCondition) MergeDuplicates(data []interface{}) interface{} {
	totalRoles := make([]int64, 0, 100)
	for _, dupe := range data {
		cast := dupe.(*MemberRolesConditionData)
	OUTER:
		for _, r := range cast.Roles {
			for _, existing := range totalRoles {
				if r == existing {
					continue OUTER
				}
			}

			// not added
			totalRoles = append(totalRoles, r)
		}
	}

	return &MemberRolesConditionData{Roles: totalRoles}
}

/////////////////////////////////////////////////////////////////

type ChannelsConditionData struct {
	Channels []int64
}

var _ Condition = (*ChannelsCondition)(nil)

type ChannelsCondition struct {
	Blacklist bool // if true, then blacklist mode, otherwise whitelist mode
}

func (cd *ChannelsCondition) Kind() RulePartType {
	return RulePartCondition
}

func (cd *ChannelsCondition) DataType() interface{} {
	return &ChannelsConditionData{}
}

func (cd *ChannelsCondition) Name() string {
	if cd.Blacklist {
		return "Ignore channels"
	}

	return "Active in channels"
}

func (cd *ChannelsCondition) Description() string {
	if cd.Blacklist {
		return "Ignore the following channels"
	}

	return "Only check the following channels"
}

func (cd *ChannelsCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "Channels",
			Key:  "Channels",
			Kind: SettingTypeMultiChannel,
		},
	}
}

func (cd *ChannelsCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*ChannelsConditionData)
	if data.CS == nil {
		return true, nil
	}

	if common.ContainsInt64Slice(settingsCast.Channels, data.CS.ID) {
		if cd.Blacklist {
			// Blacklisted channel
			return false, nil
		} else {
			// Whilelisted channel
			return true, nil
		}
	}

	if cd.Blacklist {
		// Not in a blacklisted channel
		return true, nil
	}

	// Not in a whitelisted channel
	return false, nil
}

func (cd *ChannelsCondition) MergeDuplicates(data []interface{}) interface{} {
	totalChannels := make([]int64, 0, 100)
	for _, dupe := range data {
		cast := dupe.(*ChannelsConditionData)
	OUTER:
		for _, c := range cast.Channels {
			for _, existing := range totalChannels {
				if c == existing {
					continue OUTER
				}
			}

			// not added
			totalChannels = append(totalChannels, c)
		}
	}

	return &ChannelsConditionData{Channels: totalChannels}
}

/////////////////////////////////////////////////////////////////

type ChannelCategoryConditionData struct {
	Categories []int64
}

var _ Condition = (*ChannelCategoriesCondition)(nil)

type ChannelCategoriesCondition struct {
	Blacklist bool // if true, then blacklist mode, otherwise whitelist mode
}

func (cd *ChannelCategoriesCondition) Kind() RulePartType {
	return RulePartCondition
}

func (cd *ChannelCategoriesCondition) DataType() interface{} {
	return &ChannelCategoryConditionData{}
}

func (cd *ChannelCategoriesCondition) Name() string {
	if cd.Blacklist {
		return "Ignore categories"
	}

	return "Active in categories"
}

func (cd *ChannelCategoriesCondition) Description() string {
	if cd.Blacklist {
		return "Ignore channels in the following categories"
	}

	return "Only check channels in the following categories"
}

func (cd *ChannelCategoriesCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "Categories",
			Key:  "Categories",
			Kind: SettingTypeMultiChannelCategories,
		},
	}
}

func (cd *ChannelCategoriesCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*ChannelCategoryConditionData)
	if data.CS == nil {
		return true, nil
	}

	if common.ContainsInt64Slice(settingsCast.Categories, data.CS.ParentID) {
		if cd.Blacklist {
			// blacklisted channel category
			return false, nil
		} else {
			// whilelisted channel category
			return true, nil
		}
	}

	if cd.Blacklist {
		// not in a blacklisted channel category
		return true, nil
	}

	// not in a whitelisted channel category
	return false, nil
}

func (cd *ChannelCategoriesCondition) MergeDuplicates(data []interface{}) interface{} {
	totalCats := make([]int64, 0, 100)
	for _, dupe := range data {
		cast := dupe.(*ChannelCategoryConditionData)
	OUTER:
		for _, c := range cast.Categories {
			for _, existing := range totalCats {
				if c == existing {
					continue OUTER
				}
			}

			// not added
			totalCats = append(totalCats, c)
		}
	}

	return &ChannelCategoryConditionData{Categories: totalCats}
}

/////////////////////////////////////////////////////////////////

type AccountAgeConditionData struct {
	Treshold int
}

var _ Condition = (*AccountAgeCondition)(nil)

type AccountAgeCondition struct {
	Below bool // if true, then blacklist mode, otherwise whitelist mode
}

func (ac *AccountAgeCondition) Kind() RulePartType {
	return RulePartCondition
}

func (ac *AccountAgeCondition) DataType() interface{} {
	return &AccountAgeConditionData{}
}

func (ac *AccountAgeCondition) Name() string {
	if ac.Below {
		return "Account age below"
	}

	return "Account age above"
}

func (ac *AccountAgeCondition) Description() string {
	if ac.Below {
		return "Ignore users whose accounts age is greater than the specified threshold"
	}

	return "Ignore users whose accounts age is less than the specified threshold"
}

func (ac *AccountAgeCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "Age in minutes",
			Key:  "Treshold",
			Kind: SettingTypeInt,
		},
	}
}

func (ac *AccountAgeCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*AccountAgeConditionData)

	created := bot.SnowflakeToTime(data.MS.ID)
	minutes := int(time.Since(created).Minutes())
	if minutes <= settingsCast.Treshold {
		// account were made within threshold
		if ac.Below {
			return true, nil
		} else {
			return false, nil
		}
	}

	// account is older than threshold
	if ac.Below {
		return false, nil
	} else {
		return true, nil
	}
}

func (ac *AccountAgeCondition) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////////

type MemberAgeConditionData struct {
	Treshold int
}

var _ Condition = (*MemberAgecondition)(nil)

type MemberAgecondition struct {
	Below bool // if true, then blacklist mode, otherwise whitelist mode
}

func (mc *MemberAgecondition) Kind() RulePartType {
	return RulePartCondition
}

func (mc *MemberAgecondition) DataType() interface{} {
	return &MemberAgeConditionData{}
}

func (mc *MemberAgecondition) Name() string {
	if mc.Below {
		return "Server Member duration below"
	}

	return "Server Member duration above"
}

func (mc *MemberAgecondition) Description() string {
	if mc.Below {
		return "Require members to have been on the server for less than x minutes"
	}

	return "Require members to have been on the server for more than x minutes"
}

func (mc *MemberAgecondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "Age in minutes",
			Key:  "Treshold",
			Kind: SettingTypeInt,
		},
	}
}

func (mc *MemberAgecondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*MemberAgeConditionData)

	joinedAt := data.MS.JoinedAt
	if joinedAt.IsZero() {
		newMS, err := bot.GetMemberJoinedAt(data.GS.ID, data.MS.ID)
		if err != nil {
			return false, err
		}
		joinedAt = newMS.JoinedAt
	}

	minutes := int(time.Since(joinedAt).Minutes())

	if minutes <= settingsCast.Treshold {
		// joined within threshold
		if mc.Below {
			return true, nil
		} else {
			return false, nil
		}
	}

	// joined before threshold
	if mc.Below {
		return false, nil
	} else {
		return true, nil
	}
}

func (mc *MemberAgecondition) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////////

var _ Condition = (*BotCondition)(nil)

type BotCondition struct {
	Ignore bool
}

func (bc *BotCondition) Kind() RulePartType {
	return RulePartCondition
}

func (bc *BotCondition) DataType() interface{} {
	return nil
}

func (bc *BotCondition) Name() string {
	if bc.Ignore {
		return "Ignore bots"
	}

	return "Only bots"
}

func (bc *BotCondition) Description() string {
	if bc.Ignore {
		return "Ignore all bots"
	}

	return "Ignore all other users than bots"
}

func (bc *BotCondition) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (bc *BotCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	if bc.Ignore {
		return !data.MS.Bot, nil
	}

	return data.MS.Bot, nil
}

func (bc *BotCondition) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////////

var _ Condition = (*MessageEditedCondition)(nil)

type MessageEditedCondition struct {
	NewMessage bool // if true, then blacklist mode, otherwise whitelist mode
}

func (mc *MessageEditedCondition) Kind() RulePartType {
	return RulePartCondition
}

func (mc *MessageEditedCondition) DataType() interface{} {
	return nil
}

func (mc *MessageEditedCondition) Name() string {
	if mc.NewMessage {
		return "New message"
	}
	return "Edited message"
}

func (mc *MessageEditedCondition) Description() string {
	if mc.NewMessage {
		return "Ignore edited messages"
	}
	return "Only examine edited messages"
}

func (mc *MessageEditedCondition) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (mc *MessageEditedCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	if data.Message == nil {
		// pass the condition if no message is found
		return true, nil
	}
	if data.Message.EditedTimestamp == "" {
		// new post
		if mc.NewMessage {
			return true, nil
		}
		return false, nil
	}

	// message was edited
	if mc.NewMessage {
		return false, nil
	}
	return true, nil
}

func (mc *MessageEditedCondition) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}
