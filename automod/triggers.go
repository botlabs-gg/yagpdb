package automod

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/sirupsen/logrus"
	"regexp"
	"time"
)

type MentionsTriggerData struct {
	Treshold int
}

var _ MessageTrigger = (*MentionsTrigger)(nil)

type MentionsTrigger struct{}

func (mc *MentionsTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (mc *MentionsTrigger) DataType() interface{} {
	return &MentionsTriggerData{}
}

func (mc *MentionsTrigger) Name() string {
	return "Mentions"
}

func (mc *MentionsTrigger) Description() string {
	return "Triggers when a message includes more than x unique mentions."
}

func (mc *MentionsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Treshold",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 4,
		},
	}
}

func (mc *MentionsTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, data interface{}) (bool, error) {
	dataCast := data.(*MentionsTriggerData)
	if len(m.Mentions) > dataCast.Treshold {
		return true, nil
	}

	return false, nil
}

var _ MessageTrigger = (*AnyLinkTrigger)(nil)

type AnyLinkTrigger struct{}

func (alc *AnyLinkTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (alc *AnyLinkTrigger) DataType() interface{} {
	return nil
}

func (alc *AnyLinkTrigger) Name() (name string) {
	return "Any Link"
}

func (alc *AnyLinkTrigger) Description() (description string) {
	return "Triggers when any link is sent."
}

func (alc *AnyLinkTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

var LinkRegex = regexp.MustCompile(`((https?|steam):\/\/[^\s<]+[^<.,:;"')\]\s])`)

func (alc *AnyLinkTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, data interface{}) (bool, error) {
	if LinkRegex.MatchString(m.ContentWithMentionsReplaced()) {
		return true, nil
	}

	logrus.Println("Mathced trigger: ", m.ContentWithMentionsReplaced())

	return false, nil
}

type ViolationsTriggerData struct {
	Name     string `valid:",1,100,trimspace"`
	Treshold int
	Interval int
}

var _ ViolationListener = (*ViolationsTrigger)(nil)

type ViolationsTrigger struct{}

func (vt *ViolationsTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (vt *ViolationsTrigger) DataType() interface{} {
	return &ViolationsTriggerData{}
}

func (vt *ViolationsTrigger) Name() string {
	return "x Violations in x minutes"
}

func (vt *ViolationsTrigger) Description() string {
	return "Triggers when a user has more than x violations within x minutes."
}

func (vt *ViolationsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Violation name",
			Key:     "Name",
			Kind:    SettingTypeString,
			Default: "violation name",
			Min:     1,
			Max:     50,
		},
		&SettingDef{
			Name:    "Number of violations",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 4,
		},
		&SettingDef{
			Name:    "Within (minutes)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: 60,
		},
	}
}

func (vt *ViolationsTrigger) CheckUser(ms *dstate.MemberState, gs *dstate.GuildState, violations []*models.AutomodViolation, data interface{}) (isAffected bool, err error) {
	dataCast := data.(*ViolationsTriggerData)
	numRecent := 0
	for _, v := range violations {
		if v.Name != dataCast.Name {
			continue
		}

		if time.Since(v.CreatedAt).Minutes() > float64(dataCast.Interval) {
			continue
		}

		numRecent++
	}

	if numRecent >= dataCast.Treshold {
		return true, nil
	}

	return false, nil
}
