package automod

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
	"strings"
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
	13: &WordListTrigger{Blacklist: true},
	14: &WordListTrigger{Blacklist: false},
	15: &DomainTrigger{Blacklist: true},
	16: &DomainTrigger{Blacklist: false},
	17: &AllCapsTrigger{},
	18: &ChannelCategoriesCondition{Blacklist: true},
	19: &ChannelCategoriesCondition{Blacklist: false},
	20: &ServerInviteTrigger{},
	21: &AccountAgeCondition{Below: false},
	22: &AccountAgeCondition{Below: true},
	23: &MemberAgecondition{Below: false},
	24: &MemberAgecondition{Below: true},
	25: &WarnUserEffect{},
	26: &GoogleSafeBrowsingTrigger{},
	27: &BotCondition{Ignore: true},
	28: &SetNicknameEffect{},
	29: &SlowmodeTrigger{ChannelBased: false},
	30: &SlowmodeTrigger{ChannelBased: true},
	31: &MultiMsgMentionTrigger{ChannelBased: false},
	32: &MultiMsgMentionTrigger{ChannelBased: true},
}

var InverseRulePartMap = make(map[RulePart]int)

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

		InverseRulePartMap[v] = k
	}
}

type SettingType string

const (
	SettingTypeRole                   = "role"
	SettingTypeMultiRole              = "multi_role"
	SettingTypeChannel                = "channel"
	SettingTypeMultiChannel           = "multi_channel"
	SettingTypeMultiChannelCategories = "multi_channel_cat"
	SettingTypeInt                    = "int"
	SettingTypeString                 = "string"
	SettingTypeBool                   = "bool"
	SettingTypeList                   = "list"
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

type MergeableRulePart interface {
	MergeDuplicates(data []interface{}) interface{}
}

type TriggeredRuleData struct {
	// Should always be available
	Plugin  *Plugin
	GS      *dstate.GuildState
	MS      *dstate.MemberState
	Ruleset *ParsedRuleset

	// not present when checking rs conditions
	CurrentRule *ParsedRule

	// not present when checking conditions
	TriggeredRules    []*ParsedRule
	ActivatedTriggers []*ParsedPart

	// Optional data that may not be present
	CS      *dstate.ChannelState
	Message *discordgo.Message

	RecursionCounter int

	// Gets added to when we recurse using +violation
	PreviousReasons []string
}

func (t *TriggeredRuleData) Clone() *TriggeredRuleData {
	n := *t
	n.PreviousReasons = make([]string, len(t.PreviousReasons))
	copy(n.PreviousReasons, t.PreviousReasons)
	return &n
}

func (t *TriggeredRuleData) ConstructReason(includePrevious bool) string {
	var builder strings.Builder
	if includePrevious {
		for _, previous := range t.PreviousReasons {
			builder.WriteString(previous)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("Triggered rule: ")
	if t.CurrentRule == nil {
		builder.WriteString("unknown rule?`")
	} else {
		builder.WriteString(t.CurrentRule.Model.Name)

		for _, p := range t.ActivatedTriggers {
			if p.RuleModel.RuleID == t.CurrentRule.Model.ID {
				builder.WriteString(" (`" + p.Part.Name() + "`)")
				break
			}
		}
	}

	return builder.String()
}

// MessageCondition is a active condition that needs to run on a message
type MessageTrigger interface {
	RulePart

	CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, data interface{}) (isAffected bool, err error)
}

// ViolationListener is a trigger that gets triggered on a violation
type ViolationListener interface {
	RulePart

	CheckUser(ctxData *TriggeredRuleData, violations []*models.AutomodViolation, data interface{}, triggeredOnHigher bool) (isAffected bool, err error)
}
