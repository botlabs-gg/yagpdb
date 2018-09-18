package automod

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
)

// maps rule part indentifiers to actual condition types
// since these are stored in the database, changing the id's would require an update of all the relevant rows
// so don't do that.
var RulePartMap = map[int]RulePart{
	1:  &MemberRolesCondition{Blacklist: true},
	2:  &MemberRolesCondition{Blacklist: false},
	3:  &MentionsTrigger{},
	4:  &AnyLinkTrigger{},
	5:  &DeleteMessageEffect{},
	6:  &AddViolationEffect{},
	7:  &ChannelsCondition{Blacklist: true},
	8:  &ChannelsCondition{Blacklist: false},
	9:  &KickUserEffect{},
	10: &BanUserEffect{},
	11: &MuteUserEffect{},
	12: &ViolationsTrigger{},
}

// helpers for querying the db
var messageTriggers []interface{}
var violationTriggers []interface{}

func init() {
	for k, v := range RulePartMap {
		if _, ok := v.(MessageTrigger); ok {
			messageTriggers = append(messageTriggers, k)
		}

		if _, ok := v.(ViolationListener); ok {
			violationTriggers = append(violationTriggers, k)
		}
	}
}

type SettingType string

const (
	SettingTypeRole         = "role"
	SettingTypeMultiRole    = "multi_role"
	SettingTypeChannel      = "channel"
	SettingTypeMultiChannel = "multi_channel"
	SettingTypeInt          = "int"
	SettingTypeString       = "string"
	SettingTypeBool         = "bool"
)

type SettingDef struct {
	Name     string
	Key      string
	Kind     SettingType
	Min, Max int
	Default  interface{} `json:",omitempty"`
}

type RulePartType int

const (
	RulePartTrigger   RulePartType = 0
	RulePartCondition RulePartType = 1
	RulePartEffect    RulePartType = 2
)

// RulePart represents a single condition, trigger or effect
type RulePart interface {
	// Datatype needs to return a new object to unmarshal the settings into, if there is none for this rule data entry then return nil
	DataType() interface{}

	// Returns the available user settings that can be changed (such as roles)
	UserSettings() []*SettingDef

	// Returns a human readble name for this rule data entry and a description
	Name() string
	Description() string

	Kind() RulePartType
}

type TriggeredRuleData struct {
	Plugin  *Plugin
	GS      *dstate.GuildState
	MS      *dstate.MemberState
	Ruleset *ParsedRuleset
	Rule    *ParsedRule

	// Optional data that may not be present
	CS      *dstate.ChannelState
	Message *discordgo.Message
}

// MessageCondition is a active condition that needs to run on a message
type MessageTrigger interface {
	RulePart

	CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, data interface{}) (isAffected bool, err error)
}

// ViolationListener is a trigger that gets triggered on a violation
type ViolationListener interface {
	RulePart

	CheckUser(ms *dstate.MemberState, gs *dstate.GuildState, violations []*models.AutomodViolation, data interface{}) (isAffected bool, err error)
}
