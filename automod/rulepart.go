package automod

import (
	"sort"
	"strings"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v2"
	"github.com/jonas747/yagpdb/automod/models"
)

// maps rule part indentifiers to actual condition types
// since these are stored in the database, changing the id's would require an update of all the relevant rows
// so don't do that.
var RulePartMap = map[int]RulePart{
	// Triggers <2xx
	1:  &AllCapsTrigger{},
	2:  &MentionsTrigger{},
	3:  &AnyLinkTrigger{},
	4:  &ViolationsTrigger{},
	5:  &WordListTrigger{Blacklist: true},
	6:  &WordListTrigger{Blacklist: false},
	7:  &DomainTrigger{Blacklist: true},
	8:  &DomainTrigger{Blacklist: false},
	9:  &ServerInviteTrigger{},
	10: &GoogleSafeBrowsingTrigger{},
	11: &SlowmodeTrigger{ChannelBased: false},
	12: &SlowmodeTrigger{ChannelBased: true},
	13: &MultiMsgMentionTrigger{ChannelBased: false},
	14: &MultiMsgMentionTrigger{ChannelBased: true},
	15: &MessageRegexTrigger{},
	16: &MessageRegexTrigger{BaseRegexTrigger: BaseRegexTrigger{Inverse: true}},
	17: &SpamTrigger{},
	18: &NicknameRegexTrigger{BaseRegexTrigger: BaseRegexTrigger{Inverse: false}},
	19: &NicknameRegexTrigger{BaseRegexTrigger: BaseRegexTrigger{Inverse: true}},
	20: &NicknameWordlistTrigger{Blacklist: false},
	21: &NicknameWordlistTrigger{Blacklist: true},
	22: &SlowmodeTrigger{Attachments: true, ChannelBased: false},
	23: &SlowmodeTrigger{Attachments: true, ChannelBased: true},
	24: &UsernameWordlistTrigger{Blacklist: false},
	25: &UsernameWordlistTrigger{Blacklist: true},
	26: &UsernameRegexTrigger{BaseRegexTrigger{Inverse: false}},
	27: &UsernameRegexTrigger{BaseRegexTrigger{Inverse: true}},
	29: &UsernameInviteTrigger{},
	30: &MemberJoinTrigger{},
	31: &MessageAttachmentTrigger{},
	32: &MessageAttachmentTrigger{RequiresAttachment: true},

	// Conditions 2xx
	200: &MemberRolesCondition{Blacklist: true},
	201: &MemberRolesCondition{Blacklist: false},
	202: &ChannelsCondition{Blacklist: true},
	203: &ChannelsCondition{Blacklist: false},
	204: &AccountAgeCondition{Below: false},
	205: &AccountAgeCondition{Below: true},
	206: &MemberAgecondition{Below: false},
	207: &MemberAgecondition{Below: true},
	209: &BotCondition{Ignore: true},
	210: &BotCondition{Ignore: false},
	211: &ChannelCategoriesCondition{Blacklist: true},
	212: &ChannelCategoriesCondition{Blacklist: false},
	213: &MessageEditedCondition{NewMessage: true},
	214: &MessageEditedCondition{NewMessage: false},

	// Effects 3xx
	300: &DeleteMessageEffect{},
	301: &AddViolationEffect{},
	302: &KickUserEffect{},
	303: &BanUserEffect{},
	304: &MuteUserEffect{},
	305: &WarnUserEffect{},
	306: &SetNicknameEffect{},
	307: &ResetViolationsEffect{},
	308: &DeleteMessagesEffect{},
	309: &GiveRoleEffect{},
	311: &EnableChannelSlowmodeEffect{},
	312: &RemoveRoleEffect{},
}

var InverseRulePartMap = make(map[RulePart]int)

type RulePartPair struct {
	ID   int
	Part RulePart
}

var RulePartList = make([]*RulePartPair, 0)

func init() {
	for k, v := range RulePartMap {
		InverseRulePartMap[v] = k

		RulePartList = append(RulePartList, &RulePartPair{
			ID:   k,
			Part: v,
		})
	}

	sort.Slice(RulePartList, func(i, j int) bool {
		return RulePartList[i].ID < RulePartList[j].ID
	})
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
	CS                     *dstate.ChannelState
	Message                *discordgo.Message
	StrippedMessageContent string // message content stripped of markdown

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

	CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (isAffected bool, err error)
}

// ViolationListener is a trigger that gets triggered on a violation
type ViolationListener interface {
	RulePart

	CheckUser(ctxData *TriggeredRuleData, violations []*models.AutomodViolation, data interface{}, triggeredOnHigher bool) (isAffected bool, err error)
}

// NicknameListener is a trigger that gets triggered on a nickname change
type NicknameListener interface {
	RulePart

	CheckNickname(ms *dstate.MemberState, data interface{}) (isAffected bool, err error)
}

// UsernameListener is a trigger that gets triggered on a nickname change
type UsernameListener interface {
	RulePart

	CheckUsername(ms *dstate.MemberState, data interface{}) (isAffected bool, err error)
}

// JoinListener is triggers that does stuff when members joins
type JoinListener interface {
	RulePart

	CheckJoin(ms *dstate.MemberState, data interface{}) (isAffected bool, err error)
}
