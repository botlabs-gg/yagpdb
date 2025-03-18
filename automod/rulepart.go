package automod

import (
	"sort"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/automod/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
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
	24: &GlobalnameWordlistTrigger{Blacklist: false},
	25: &GlobalnameWordlistTrigger{Blacklist: true},
	26: &GlobalnameRegexTrigger{BaseRegexTrigger{Inverse: false}},
	27: &GlobalnameRegexTrigger{BaseRegexTrigger{Inverse: true}},
	29: &GlobalnameInviteTrigger{},
	30: &MemberJoinTrigger{},
	31: &MessageAttachmentTrigger{},
	32: &MessageAttachmentTrigger{RequiresAttachment: true},
	33: &AntiPhishingLinkTrigger{},
	34: &MessageLengthTrigger{},
	35: &MessageLengthTrigger{Inverted: true},
	36: &SlowmodeTrigger{Links: true, ChannelBased: false},
	37: &SlowmodeTrigger{Links: true, ChannelBased: true},
	38: &AutomodExecution{},

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
	215: &ThreadCondition{true},
	216: &ThreadCondition{false},
	217: &MessageAttachmentCondition{true},
	218: &MessageAttachmentCondition{false},
	219: &MessageForwardCondition{true},
	220: &MessageForwardCondition{false},

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
	313: &SendChannelMessageEffect{},
	314: &TimeoutUserEffect{},
	315: &SendModeratorAlertMessageEffect{},
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
	Name        string
	Key         string
	Kind        SettingType
	Min, Max    int
	Default     interface{} `json:",omitempty"`
	Placeholder string      `json:",omitempty"`
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
	GS      *dstate.GuildSet
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

type TriggerContext struct {
	GS   *dstate.GuildSet
	MS   *dstate.MemberState
	Data interface{}
}

// MessageCondition is a active condition that needs to run on a message
type MessageTrigger interface {
	RulePart

	CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (isAffected bool, err error)
}

// ViolationListener is a trigger that gets triggered on a violation
type ViolationListener interface {
	RulePart

	CheckUser(ctxData *TriggeredRuleData, violations []*models.AutomodViolation, data interface{}, triggeredOnHigher bool) (isAffected bool, err error)
}

// NicknameListener is a trigger that gets triggered on a nickname change
type NicknameListener interface {
	RulePart

	CheckNickname(triggerCtx *TriggerContext) (isAffected bool, err error)
}

// GlobalnameListener is a trigger that gets triggered when a member joins
type GlobalnameListener interface {
	RulePart

	CheckGlobalname(triggerCtx *TriggerContext) (isAffected bool, err error)
}

// JoinListener is triggers that does stuff when members joins
type JoinListener interface {
	RulePart

	CheckJoin(triggerCtx *TriggerContext) (isAffected bool, err error)
}

// AutomodListener is a trigger for when Discord's built in automod kicks in
type AutomodListener interface {
	RulePart

	CheckRuleID(triggerCtx *TriggerContext, ruleID int64) (isAffected bool, err error)
}
