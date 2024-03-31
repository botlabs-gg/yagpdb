package automod

import (
	"time"

	"github.com/botlabs-gg/quackpdb/v2/bot"
	"github.com/botlabs-gg/quackpdb/v2/common"
)

type Condition interface {
	RulePart

	// IsMet is called to check wether this condition is met or not
	IsMet(data *TriggeredRuleData, parsedSettings interface{}) (bool, error)
}

/////////////////////////////////////////////////////////////////

type MemberRolesConditionData struct {
	Roles           []int64
	RequireAllRoles bool
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
		return "Quackgnore roles"
	}

	return "Requackre roles"
}

func (mrc *MemberRolesCondition) Description() string {
	if mrc.Blacklist {
		return "Quackgnore qusers with at least one of these quroles from this qule"
	}

	return "Requackre at least one of these quroles on the quser"
}

func (mrc *MemberRolesCondition) UserSettings() []*SettingDef {
	settings := []*SettingDef{
		{
			Name: "Roles",
			Key:  "Roles",
			Kind: SettingTypeMultiRole,
		},
	}
	if !mrc.Blacklist {
		settings = append(settings, &SettingDef{
			Name:    "Requackre all selquackted roles",
			Key:     "RequireAllRoles",
			Kind:    SettingTypeBool,
			Default: false,
		})
	}
	return settings
}

func (mrc *MemberRolesCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*MemberRolesConditionData)
	allRolesPresent := false
	for _, r := range settingsCast.Roles {
		if common.ContainsInt64Slice(data.MS.Member.Roles, r) {
			if mrc.Blacklist {
				// Had a blacklist role, this condition is not met
				return false, nil
			} else if !settingsCast.RequireAllRoles {
				// Had a whitelist role, this condition is met
				return true, nil
			}
			allRolesPresent = true
		} else if settingsCast.RequireAllRoles {
			// One of the required roles is not present for the member
			return false, nil
		}
	}

	if allRolesPresent {
		return true, nil
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
		return "Quackgnore quacknnels"
	}

	return "Quacktive in quacknnels"
}

func (cd *ChannelsCondition) Description() string {
	if cd.Blacklist {
		return "Quackgnore the follquacking quacknnels"
	}

	return "Only check the follquacking quacknnels"
}

func (cd *ChannelsCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Quacknnels",
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

	if common.ContainsInt64Slice(settingsCast.Channels, common.ChannelOrThreadParentID(data.CS)) {
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
		return "Quackgnore quackegories"
	}

	return "Quacktive in quackegories"
}

func (cd *ChannelCategoriesCondition) Description() string {
	if cd.Blacklist {
		return "Quackgnore quacknnels in the follquacking quackegories"
	}

	return "Only check quacknnels in the follquacking quackegories"
}

func (cd *ChannelCategoriesCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Quackegories",
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

	// fetch thread parent if needed
	parentID := data.CS.ParentID
	if data.CS.Type.IsThread() {
		threadParent := data.GS.GetChannel(data.CS.ParentID)
		if threadParent == nil {
			return false, nil
		}

		parentID = threadParent.ParentID
	}

	if common.ContainsInt64Slice(settingsCast.Categories, parentID) {
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
		return "Quaccount age below"
	}

	return "Quaccount age above"
}

func (ac *AccountAgeCondition) Description() string {
	if ac.Below {
		return "Quackgnore qusers whose quaccounts age is greater than the specifquacked threshquack"
	}

	return "Quackgnore qusers whose quaccounts age is less than the specifquacked threshquack"
}

func (ac *AccountAgeCondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Quage in minquacks",
			Key:  "Treshold",
			Kind: SettingTypeInt,
		},
	}
}

func (ac *AccountAgeCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*AccountAgeConditionData)

	created := bot.SnowflakeToTime(data.MS.User.ID)
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
		return "Servquack Quackber quackration below"
	}

	return "Servquack Quackber quackration above"
}

func (mc *MemberAgecondition) Description() string {
	if mc.Below {
		return "Requackre quackbers to have been on the servquack for less than x minquacks"
	}

	return "Requackre quackbers to have been on the servquack for more than x minquacks"
}

func (mc *MemberAgecondition) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Age in minquacks",
			Key:  "Treshold",
			Kind: SettingTypeInt,
		},
	}
}

func (mc *MemberAgecondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	settingsCast := settings.(*MemberAgeConditionData)

	var joinedAt time.Time
	if data.MS.Member != nil && data.MS.Member.JoinedAt != "" {
		joinedAt, _ = data.MS.Member.JoinedAt.Parse()
	} else {
		newMS, err := bot.GetMember(data.GS.ID, data.MS.User.ID)
		if err != nil {
			return false, err
		}

		if newMS.Member != nil {
			joinedAt, _ = newMS.Member.JoinedAt.Parse()
		} else {
			return false, nil
		}
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
		return "Quackgnore bots"
	}

	return "Only bots"
}

func (bc *BotCondition) Description() string {
	if bc.Ignore {
		return "Quackgnore all bots"
	}

	return "Quackgnore all other qusers than bots"
}

func (bc *BotCondition) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (bc *BotCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	if bc.Ignore {
		return !data.MS.User.Bot, nil
	}

	return data.MS.User.Bot, nil
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
		return "New messquack"
	}
	return "Edited messquack"
}

func (mc *MessageEditedCondition) Description() string {
	if mc.NewMessage {
		return "Only quaxamine new messquacks"
	}
	return "Only quaxamine edited messquacks"
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

/////////////////////////////////////////////////////////////////

var _ Condition = (*ThreadCondition)(nil)

type ThreadCondition struct {
	Threads bool
}

func (bc *ThreadCondition) Kind() RulePartType {
	return RulePartCondition
}

func (bc *ThreadCondition) DataType() interface{} {
	return nil
}

func (bc *ThreadCondition) Name() string {
	if !bc.Threads {
		return "Quackgnore thrquacks"
	}

	return "Quacktive in thrquacks"
}

func (bc *ThreadCondition) Description() string {
	if !bc.Threads {
		return "Ignores quackages in thrquacks"
	}

	return "Only match quackages in thrquacks"
}

func (bc *ThreadCondition) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (bc *ThreadCondition) IsMet(data *TriggeredRuleData, settings interface{}) (bool, error) {
	//Channel won't be present in case the trigger is on member update event like nick update
	if data.CS == nil {
		return true, nil
	}

	if !bc.Threads {
		return !data.CS.Type.IsThread(), nil
	}

	return data.CS.Type.IsThread(), nil
}

func (bc *ThreadCondition) MergeDuplicates(data []interface{}) interface{} {
	return data[0]
}
