package automod

import (
	"github.com/jonas747/yagpdb/common"
)

type Condition interface {
	RulePart

	// IsMet is called to check wether this condition is met or not
	IsMet(data *TriggeredRuleData, parsedSettings interface{}) (bool, error)
}

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
		return "Role blacklist"
	}

	return "Role whitelist"

}

func (mrc *MemberRolesCondition) Description() string {
	if mrc.Blacklist {
		return "Ignore users with these roles from this rule"
	}

	return "Require one of these roles on the user"
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
		return "Channel blacklist"
	}

	return "Channel whitelist"

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
