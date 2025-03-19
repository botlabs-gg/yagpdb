package automod

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/antiphishing"
	"github.com/botlabs-gg/yagpdb/v2/automod/models"
	"github.com/botlabs-gg/yagpdb/v2/automod_legacy"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/safebrowsing"
)

var SanitizeTextName = "Also match visually similar characters such as \"Ĥéĺĺó\""

/////////////////////////////////////////////////////////////

type BaseRegexTriggerData struct {
	Regex        string `valid:",1,250"`
	SanitizeText bool
}

type BaseRegexTrigger struct {
	Inverse bool
}

func (r BaseRegexTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (r BaseRegexTrigger) DataType() interface{} {
	return &BaseRegexTriggerData{}
}

func (r BaseRegexTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Regex",
			Key:  "Regex",
			Kind: SettingTypeString,
			Min:  1,
			Max:  250,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

//////////////

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
	return "Message mentions"
}

func (mc *MentionsTrigger) Description() string {
	return "Triggers when a message includes x unique mentions."
}

func (mc *MentionsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Threshold",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 4,
		},
	}
}

func (mc *MentionsTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*MentionsTriggerData)
	if len(m.Mentions) >= dataCast.Treshold {
		return true, nil
	}

	return false, nil
}

func (mc *MentionsTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

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
	return "Triggers when a message contains any valid link"
}

func (alc *AnyLinkTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (alc *AnyLinkTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	for _, content := range m.GetMessageContents() {
		content = confusables.NormalizeQueryEncodedText(content)
		if common.LinkRegex.MatchString(common.ForwardSlashReplacer.Replace(content)) {
			return true, nil
		}
	}
	return false, nil

}

func (alc *AnyLinkTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*WordListTrigger)(nil)

type WordListTrigger struct {
	Blacklist bool
}
type WorldListTriggerData struct {
	ListID       int64
	SanitizeText bool
}

func (wl *WordListTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (wl *WordListTrigger) DataType() interface{} {
	return &WorldListTriggerData{}
}

func (wl *WordListTrigger) Name() (name string) {
	if wl.Blacklist {
		return "Word denylist"
	}

	return "Word allowlist"
}

func (wl *WordListTrigger) Description() (description string) {
	if wl.Blacklist {
		return "Triggers on messages containing words in the specified list"
	}

	return "Triggers on messages containing words not in the specified list"
}

func (wl *WordListTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

func (wl *WordListTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*WorldListTriggerData)

	list, err := FindFetchGuildList(triggerCtx.GS.ID, dataCast.ListID)
	if err != nil {
		return false, nil
	}
	var messageFields []string
	for _, content := range m.GetMessageContents() {
		content := PrepareMessageForWordCheck(content)
		messageFields = append(messageFields, strings.Fields(content)...)
		if dataCast.SanitizeText {
			messageFields = append(messageFields, strings.Fields(confusables.SanitizeText(content))...) // Could be turned into a 1-liner, lmk if I should or not
		}
	}

	for _, mf := range messageFields {
		contained := false
		for _, w := range list.Content {
			if strings.EqualFold(mf, w) {
				if wl.Blacklist {
					// contains a blacklisted word, trigger
					return true, nil
				} else {
					contained = true
					break
				}
			}
		}

		if !wl.Blacklist && !contained {
			// word not whitelisted, trigger
			return true, nil
		}
	}

	// did not contain a blacklisted word, or contained just whitelisted words
	return false, nil
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*DomainTrigger)(nil)

type DomainTrigger struct {
	Blacklist bool
}
type DomainTriggerData struct {
	ListID int64
}

func (dt *DomainTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (dt *DomainTrigger) DataType() interface{} {
	return &DomainTriggerData{}
}

func (dt *DomainTrigger) Name() (name string) {
	if dt.Blacklist {
		return "Website denylist"
	}

	return "Website allowlist"
}

func (dt *DomainTrigger) Description() (description string) {
	if dt.Blacklist {
		return "Triggers on messages containing links to websites in the specified list"
	}

	return "Triggers on messages containing links to websites NOT in the specified list"
}

func (dt *DomainTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
	}
}

func (dt *DomainTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*DomainTriggerData)

	list, err := FindFetchGuildList(triggerCtx.GS.ID, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	var matches []string
	for _, content := range m.GetMessageContents() {
		content = confusables.NormalizeQueryEncodedText(content)
		snapshotMatches := common.LinkRegex.FindAllString(common.ForwardSlashReplacer.Replace(content), -1)
		matches = append(matches, snapshotMatches...)
	}

	for _, v := range matches {
		if contains, _ := dt.containsDomain(v, list.Content); contains {
			if dt.Blacklist {
				return true, nil
			}
		} else if !dt.Blacklist {
			// whitelist mode, unknown link
			return true, nil
		}

	}

	// did not contain any link, or no blacklisted links
	return false, nil
}

func (dt *DomainTrigger) containsDomain(link string, list []string) (bool, string) {
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "steam://") {
		link = "http://" + link
	}

	parsed, err := url.ParseRequestURI(link)
	if err != nil {
		logger.WithError(err).WithField("url", link).Error("Failed parsing request url matched with regex")
		return false, ""
	}

	host := parsed.Host
	if index := strings.Index(host, ":"); index > -1 {
		host = host[:index]
	}

	host = strings.ToLower(host)

	for _, v := range list {
		if strings.HasSuffix(host, "."+v) {
			return true, v
		}

		if v == host {
			return true, v
		}
	}

	return false, ""
}

/////////////////////////////////////////////////////////////

type ViolationsTriggerData struct {
	Name           string `valid:",1,100,trimspace"`
	Treshold       int
	Interval       int
	IgnoreIfLesser bool
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
	return "x Violations in y minutes"
}

func (vt *ViolationsTrigger) Description() string {
	return "Triggers when a user has x violations within y minutes."
}

func (vt *ViolationsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Violation name",
			Key:     "Name",
			Kind:    SettingTypeString,
			Default: "name",
			Min:     1,
			Max:     50,
		},
		{
			Name:    "Number of violations",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 4,
		},
		{
			Name:    "Within (minutes)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: 60,
		},
		{
			Name:    "Ignore if a higher violation trigger of this name was activated",
			Key:     "IgnoreIfLesser",
			Kind:    SettingTypeBool,
			Default: true,
		},
	}
}

func (vt *ViolationsTrigger) CheckUser(ctxData *TriggeredRuleData, violations []*models.AutomodViolation, settings interface{}, triggeredOnHigher bool) (isAffected bool, err error) {
	settingsCast := settings.(*ViolationsTriggerData)
	if triggeredOnHigher && settingsCast.IgnoreIfLesser {
		return false, nil
	}

	numRecent := 0
	for _, v := range violations {
		if v.Name != settingsCast.Name {
			continue
		}

		if time.Since(v.CreatedAt).Minutes() > float64(settingsCast.Interval) {
			continue
		}

		numRecent++
	}

	if numRecent >= settingsCast.Treshold {
		return true, nil
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

type AllCapsTriggerData struct {
	MinLength    int
	Percentage   int
	SanitizeText bool
}

var _ MessageTrigger = (*AllCapsTrigger)(nil)

type AllCapsTrigger struct{}

func (caps *AllCapsTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (caps *AllCapsTrigger) DataType() interface{} {
	return &AllCapsTriggerData{}
}

func (caps *AllCapsTrigger) Name() string {
	return "All Caps"
}

func (caps *AllCapsTrigger) Description() string {
	return "Triggers when a message contains x% of just capitalized letters"
}

func (caps *AllCapsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Min number of all caps",
			Key:     "MinLength",
			Kind:    SettingTypeInt,
			Default: 3,
		},
		{
			Name:    "Percentage of all caps",
			Key:     "Percentage",
			Kind:    SettingTypeInt,
			Default: 100,
			Min:     1,
			Max:     100,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

func (caps *AllCapsTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*AllCapsTriggerData)

	if len(m.Content) < dataCast.MinLength {
		return false, nil
	}

	totalCapitalisableChars := 0
	numCaps := 0

	messageContent := m.Content

	if dataCast.SanitizeText {
		messageContent = confusables.SanitizeText(messageContent)
	}

	// count the number of upper case characters, note that this dosen't include other characters such as punctuation
	for _, r := range messageContent {
		if unicode.IsUpper(r) {
			numCaps++
			totalCapitalisableChars++
		} else {
			if unicode.ToUpper(r) != unicode.ToLower(r) {
				totalCapitalisableChars++
			}
		}
	}

	if totalCapitalisableChars < 1 {
		return false, nil
	}

	percentage := (numCaps * 100) / (totalCapitalisableChars)
	if numCaps >= dataCast.MinLength && percentage >= dataCast.Percentage {
		return true, nil
	}

	return false, nil
}

func (caps *AllCapsTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*ServerInviteTrigger)(nil)

type ServerInviteTrigger struct{}

func (inv *ServerInviteTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (inv *ServerInviteTrigger) DataType() interface{} {
	return nil
}

func (inv *ServerInviteTrigger) Name() string {
	return "Server invites"
}

func (inv *ServerInviteTrigger) Description() string {
	return "Triggers on messages containing invites to other servers, also includes some 3rd party server lists."
}

func (inv *ServerInviteTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (inv *ServerInviteTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	containsBadInvited := automod_legacy.CheckMessageForBadInvites(m)
	return containsBadInvited, nil
}

func (inv *ServerInviteTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*AntiPhishingLinkTrigger)(nil)

type AntiPhishingLinkTrigger struct{}

func (a *AntiPhishingLinkTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (a *AntiPhishingLinkTrigger) Name() string {
	return "Flagged Scam links"
}

func (a *AntiPhishingLinkTrigger) DataType() interface{} {
	return nil
}

func (a *AntiPhishingLinkTrigger) Description() string {
	return "Triggers on messages that have scam links flagged by SinkingYachts and BitFlow AntiPhishing APIs"
}

func (a *AntiPhishingLinkTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (a *AntiPhishingLinkTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	for _, content := range m.GetMessageContents() {
		badDomain, err := antiphishing.CheckMessageForPhishingDomains(common.ForwardSlashReplacer.Replace(content))
		if err != nil {
			logger.WithError(err).Error("Failed to check url ")
			continue
		}
		if badDomain != "" {
			return true, nil
		}
	}
	return false, nil
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*GoogleSafeBrowsingTrigger)(nil)

type GoogleSafeBrowsingTrigger struct{}

func (g *GoogleSafeBrowsingTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (g *GoogleSafeBrowsingTrigger) DataType() interface{} {
	return nil
}

func (g *GoogleSafeBrowsingTrigger) Name() string {
	return "Google flagged bad links"
}

func (g *GoogleSafeBrowsingTrigger) Description() string {
	return "Triggers on messages containing links that are flagged by Google Safebrowsing as unsafe."
}

func (g *GoogleSafeBrowsingTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (g *GoogleSafeBrowsingTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	for _, input := range m.GetMessageContents() {
		threat, err := safebrowsing.CheckString(common.ForwardSlashReplacer.Replace(input))
		if err != nil {
			logger.WithError(err).Error("Failed checking urls against google safebrowser")
			continue
		}
		if threat != nil {
			return true, nil
		}
	}
	return false, nil
}

func (g *GoogleSafeBrowsingTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

type SlowmodeTriggerData struct {
	Treshold                 int
	Interval                 int
	SingleMessageAttachments bool
	SingleMessageLinks       bool
}

var _ MessageTrigger = (*SlowmodeTrigger)(nil)

type SlowmodeTrigger struct {
	ChannelBased bool
	Attachments  bool // whether this trigger checks attachments or not
	Links        bool // whether this trigger checks links or not
}

func (s *SlowmodeTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (s *SlowmodeTrigger) DataType() interface{} {
	return &SlowmodeTriggerData{}
}

func (s *SlowmodeTrigger) Name() string {
	if s.ChannelBased {
		if s.Attachments {
			return "x channel attachments in y seconds"
		}
		if s.Links {
			return "x channel links in y seconds"
		}
		return "x channel messages in y seconds"
	}

	if s.Attachments {
		return "x user attachments in y seconds"
	}
	if s.Links {
		return "x user links in y seconds"
	}
	return "x user messages in y seconds"
}

func (s *SlowmodeTrigger) Description() string {
	if s.ChannelBased {
		if s.Attachments {
			return "Triggers when a channel has x attachments within y seconds"
		}
		if s.Links {
			return "Triggers when a channel has x links within y seconds"
		}
		return "Triggers when a channel has x messages in y seconds."
	}

	if s.Attachments {
		return "Triggers when a user has x attachments within y seconds in a single channel"
	}
	if s.Links {
		return "Triggers when a user has x links within y seconds in a single channel"
	}
	return "Triggers when a user has x messages in y seconds in a single channel."
}

func (s *SlowmodeTrigger) UserSettings() []*SettingDef {
	defaultMessages := 5
	defaultInterval := 5
	thresholdName := "Messages"

	if s.Attachments {
		defaultMessages = 10
		defaultInterval = 60
		thresholdName = "Attachments"
	} else if s.Links {
		defaultInterval = 60
		thresholdName = "Links"
	}

	settings := []*SettingDef{
		{
			Name:    thresholdName,
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: defaultMessages,
		},
		{
			Name:    "Within (seconds)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: defaultInterval,
		},
	}

	if s.Attachments {
		settings = append(settings, &SettingDef{
			Name:    "Also count multiple attachments in single message",
			Key:     "SingleMessageAttachments",
			Kind:    SettingTypeBool,
			Default: false,
		})
	} else if s.Links {
		settings = append(settings, &SettingDef{
			Name:    "Also count multiple links in single message",
			Key:     "SingleMessageLinks",
			Kind:    SettingTypeBool,
			Default: false,
		})
	}

	return settings
}

func (s *SlowmodeTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	if s.Attachments && len(m.GetMessageAttachments()) < 1 {
		return false, nil
	}
	content := confusables.NormalizeQueryEncodedText(strings.Join(m.GetMessageContents(), ""))
	if s.Links && !common.LinkRegex.MatchString(common.ForwardSlashReplacer.Replace(content)) {
		return false, nil
	}

	settings := triggerCtx.Data.(*SlowmodeTriggerData)

	within := time.Duration(settings.Interval) * time.Second
	now := time.Now()

	amount := 0

	messages := bot.State.GetMessages(cs.GuildID, cs.ID, &dstate.MessagesQuery{
		Limit: 1000,
	})

	// New messages are at the end
	for _, v := range messages {
		age := now.Sub(v.ParsedCreatedAt)
		if age > within {
			break
		}

		if !s.ChannelBased && v.Author.ID != triggerCtx.MS.User.ID {
			continue
		}

		if s.Attachments {
			vAttachments := v.GetMessageAttachments()

			if len(vAttachments) < 1 {
				continue // we're only checking messages with attachments
			}
			if settings.SingleMessageAttachments {
				// Add the count of all attachments of this message to the amount
				amount += len(vAttachments)
			} else {
				amount++
			}
		} else if s.Links {
			contents := v.GetMessageContents()
			contentString := confusables.NormalizeQueryEncodedText(strings.Join(contents, ""))
			linksLen := len(common.LinkRegex.FindAllString(common.ForwardSlashReplacer.Replace(contentString), -1))
			if linksLen < 1 {
				continue // we're only checking messages with links
			}
			if settings.SingleMessageLinks {
				// Add the count of all links of this message to the amount
				amount += linksLen
			} else {
				amount++
			}
		} else {
			amount++
		}

		if amount >= settings.Treshold {
			return true, nil
		}
	}

	return false, nil
}

func (s *SlowmodeTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

type MultiMsgMentionTriggerData struct {
	Treshold        int
	Interval        int
	CountDuplicates bool
}

var _ MessageTrigger = (*MultiMsgMentionTrigger)(nil)

type MultiMsgMentionTrigger struct {
	ChannelBased bool
}

func (mt *MultiMsgMentionTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (mt *MultiMsgMentionTrigger) DataType() interface{} {
	return &MultiMsgMentionTriggerData{}
}

func (mt *MultiMsgMentionTrigger) Name() string {
	if mt.ChannelBased {
		return "channel: x mentions within y seconds"
	}

	return "user: x mentions within y seconds"
}

func (mt *MultiMsgMentionTrigger) Description() string {
	if mt.ChannelBased {
		return "Triggers when a channel has x unique mentions in y seconds"
	}

	return "Triggers when a user has sent x unique mentions in y seconds in a single channel"
}

func (mt *MultiMsgMentionTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Mentions",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 20,
		},
		{
			Name:    "Within (seconds)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: 10,
		},
		{
			Name: "Count multiple mentions to the same user",
			Key:  "CountDuplicates",
			Kind: SettingTypeBool,
		},
	}
}

func (mt *MultiMsgMentionTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	if len(m.Mentions) < 1 {
		return false, nil
	}

	settings := triggerCtx.Data.(*MultiMsgMentionTriggerData)

	within := time.Duration(settings.Interval) * time.Second
	now := time.Now()

	mentions := make([]int64, 0)

	messages := bot.State.GetMessages(cs.GuildID, cs.ID, &dstate.MessagesQuery{
		Limit: 1000,
	})

	// New messages are at the end
	for _, v := range messages {
		age := now.Sub(v.ParsedCreatedAt)
		if age > within {
			break
		}

		if mt.ChannelBased || v.Author.ID == triggerCtx.MS.User.ID {
			// we only care about unique mentions, e.g mentioning the same user a ton wont do anythin
			for _, msgMention := range v.Mentions {
				if settings.CountDuplicates || !common.ContainsInt64Slice(mentions, msgMention.ID) {
					mentions = append(mentions, msgMention.ID)
				}
			}
		}

		if len(mentions) >= settings.Treshold {
			return true, nil
		}
	}

	if len(mentions) >= settings.Treshold {
		return true, nil
	}

	return false, nil
}

func (mt *MultiMsgMentionTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*MessageRegexTrigger)(nil)

type MessageRegexTrigger struct {
	BaseRegexTrigger
}

func (r *MessageRegexTrigger) Name() string {
	if r.BaseRegexTrigger.Inverse {
		return "Message not matching regex"
	}

	return "Message matches regex"
}

func (r *MessageRegexTrigger) Description() string {
	if r.BaseRegexTrigger.Inverse {
		return "Triggers when a message does not match the provided regex"
	}

	return "Triggers when a message matches the provided regex"
}

func (r *MessageRegexTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*BaseRegexTriggerData)

	item, err := RegexCache.Fetch(dataCast.Regex, time.Minute*10, func() (interface{}, error) {
		re, err := regexp.Compile(dataCast.Regex)
		if err != nil {
			return nil, err
		}

		return re, nil
	})

	if err != nil {
		return false, nil
	}

	re := item.Value().(*regexp.Regexp)

	for _, content := range m.GetMessageContents() {
		var sanitizedContent string
		if dataCast.SanitizeText {
			sanitizedContent = confusables.SanitizeText(content)
		}

		if re.MatchString(m.Content) || (dataCast.SanitizeText && re.MatchString(sanitizedContent)) {
			if r.BaseRegexTrigger.Inverse {
				continue
			}
			return true, nil
		}

		if r.BaseRegexTrigger.Inverse {
			return true, nil
		}
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

type SpamTriggerData struct {
	Treshold          int
	TimeLimit         int
	SanitizeText      bool
	CrossChannelMatch bool
}

var _ MessageTrigger = (*SpamTrigger)(nil)

type SpamTrigger struct{}

func (spam *SpamTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (spam *SpamTrigger) DataType() interface{} {
	return &SpamTriggerData{}
}

func (spam *SpamTrigger) Name() string {
	return "x consecutive identical messages"
}

func (spam *SpamTrigger) Description() string {
	return "Triggers when a user sends x identical messages after eachother"
}

func (spam *SpamTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name:    "Threshold",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Min:     1,
			Max:     250,
			Default: 4,
		},
		{
			Name:    "Within seconds (0 = infinity)",
			Key:     "TimeLimit",
			Kind:    SettingTypeInt,
			Min:     0,
			Max:     10000,
			Default: 30,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
		{
			Name:    "Match duplicates across channels",
			Key:     "CrossChannelMatch",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

func (spam *SpamTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {

	settingsCast := triggerCtx.Data.(*SpamTriggerData)

	mToCheckAgainst := strings.TrimSpace(strings.ToLower(m.Content))

	count := 1

	timeLimit := time.Now().Add(-time.Second * time.Duration(settingsCast.TimeLimit))

	var channelID int64 = 0
	if !settingsCast.CrossChannelMatch {
		channelID = cs.ID
	}

	messages := bot.State.GetMessages(cs.GuildID, channelID, &dstate.MessagesQuery{
		Limit: 1000,
	})

	for _, v := range messages {
		if v.ID == m.ID {
			continue
		}

		if v.Author.ID != m.Author.ID {
			continue
		}

		if settingsCast.TimeLimit > 0 && timeLimit.After(v.ParsedCreatedAt) {
			// if this message was created before the time limit, then break out
			break
		}

		if len(v.GetMessageAttachments()) > 0 {
			break // treat any attachment as a different message, in the future i may download them and check hash or something? maybe too much
		}

		contentStripped := strings.TrimSpace(v.Content)
		contentLowered := strings.ToLower(contentStripped)

		if contentLowered == mToCheckAgainst {
			count++
			continue
		}

		if !settingsCast.SanitizeText {
			continue
		}

		if confusables.SanitizeText(contentLowered) == mToCheckAgainst {
			count++
			continue
		}

		break
	}

	if count >= settingsCast.Treshold {
		return true, nil
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ NicknameListener = (*NicknameRegexTrigger)(nil)

type NicknameRegexTrigger struct {
	BaseRegexTrigger
}

func (r *NicknameRegexTrigger) Name() string {
	if r.BaseRegexTrigger.Inverse {
		return "Nickname not matching regex"
	}

	return "Nickname matches regex"
}

func (r *NicknameRegexTrigger) Description() string {
	if r.BaseRegexTrigger.Inverse {
		return "Triggers when a members nickname does not match the provided regex"
	}

	return "Triggers when a members nickname matches the provided regex"
}

func (r *NicknameRegexTrigger) CheckNickname(t *TriggerContext) (bool, error) {
	dataCast := t.Data.(*BaseRegexTriggerData)

	item, err := RegexCache.Fetch(dataCast.Regex, time.Minute*10, func() (interface{}, error) {
		re, err := regexp.Compile(dataCast.Regex)
		if err != nil {
			return nil, err
		}

		return re, nil
	})

	if err != nil {
		return false, nil
	}

	re := item.Value().(*regexp.Regexp)

	var sanitizedNick string
	if dataCast.SanitizeText {
		sanitizedNick = confusables.SanitizeText(t.MS.Member.Nick)
	}

	if re.MatchString(t.MS.Member.Nick) || (dataCast.SanitizeText && re.MatchString(sanitizedNick)) {
		if r.BaseRegexTrigger.Inverse {
			return false, nil
		}
		return true, nil
	}

	if r.BaseRegexTrigger.Inverse {
		return true, nil
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ NicknameListener = (*NicknameWordlistTrigger)(nil)

type NicknameWordlistTrigger struct {
	Blacklist bool
}
type NicknameWordlistTriggerData struct {
	ListID       int64
	SanitizeText bool
}

func (nwl *NicknameWordlistTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (nwl *NicknameWordlistTrigger) DataType() interface{} {
	return &NicknameWordlistTriggerData{}
}

func (nwl *NicknameWordlistTrigger) Name() (name string) {
	if nwl.Blacklist {
		return "Nickname word denylist"
	}

	return "Nickname word allowlist"
}

func (nwl *NicknameWordlistTrigger) Description() (description string) {
	if nwl.Blacklist {
		return "Triggers when a member has a nickname containing words in the specified list, this is currently very easy to circumvent atm, and will likely be improved in the future."
	}

	return "Triggers when a member has a nickname containing words not in the specified list, this is currently very easy to circumvent atm, and will likely be improved in the future."
}

func (nwl *NicknameWordlistTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

func (nwl *NicknameWordlistTrigger) CheckNickname(t *TriggerContext) (bool, error) {
	dataCast := t.Data.(*NicknameWordlistTriggerData)

	list, err := FindFetchGuildList(t.GS.ID, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	fields := strings.Fields(PrepareMessageForWordCheck(t.MS.Member.Nick))
	if dataCast.SanitizeText {
		messageFieldsFixText := strings.Fields(confusables.SanitizeText(PrepareMessageForWordCheck(t.MS.Member.Nick)))
		fields = append(fields, messageFieldsFixText...) // Could be turned into a 1-liner, lmk if I should or not
	}

	for _, mf := range fields {
		contained := false
		for _, w := range list.Content {
			if strings.EqualFold(mf, w) {
				if nwl.Blacklist {
					// contains a blacklisted word, trigger
					return true, nil
				} else {
					contained = true
					break
				}
			}
		}

		if !nwl.Blacklist && !contained {
			// word not whitelisted, trigger
			return true, nil
		}
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ GlobalnameListener = (*GlobalnameRegexTrigger)(nil)

type GlobalnameRegexTrigger struct {
	BaseRegexTrigger
}

func (r *GlobalnameRegexTrigger) Name() string {
	if r.BaseRegexTrigger.Inverse {
		return "Join Globalname not matching regex"
	}

	return "Join Globalname matches regex"
}

func (r *GlobalnameRegexTrigger) Description() string {
	if r.BaseRegexTrigger.Inverse {
		return "Triggers when a member joins with a Globalname that does not match the provided regex"
	}

	return "Triggers when a member joins with a Globalname that matches the provided regex"
}

func (r *GlobalnameRegexTrigger) CheckGlobalname(t *TriggerContext) (bool, error) {
	dataCast := t.Data.(*BaseRegexTriggerData)

	item, err := RegexCache.Fetch(dataCast.Regex, time.Minute*10, func() (interface{}, error) {
		re, err := regexp.Compile(dataCast.Regex)
		if err != nil {
			return nil, err
		}

		return re, nil
	})

	if err != nil {
		return false, nil
	}

	re := item.Value().(*regexp.Regexp)

	var sanitizedGlobalname string
	if dataCast.SanitizeText {
		sanitizedGlobalname = confusables.SanitizeText(t.MS.User.Globalname)
	}

	if re.MatchString(t.MS.User.Globalname) || (dataCast.SanitizeText && re.MatchString(sanitizedGlobalname)) {
		if r.BaseRegexTrigger.Inverse {
			return false, nil
		}
		return true, nil
	}

	if r.BaseRegexTrigger.Inverse {
		return true, nil
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ GlobalnameListener = (*GlobalnameWordlistTrigger)(nil)

type GlobalnameWordlistTrigger struct {
	Blacklist bool
}
type GlobalnameWorldlistData struct {
	ListID       int64
	SanitizeText bool
}

func (gwl *GlobalnameWordlistTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (gwl *GlobalnameWordlistTrigger) DataType() interface{} {
	return &GlobalnameWorldlistData{}
}

func (gwl *GlobalnameWordlistTrigger) Name() (name string) {
	if gwl.Blacklist {
		return "Join Globalname word denylist"
	}

	return "Join Globalname word allowlist"
}

func (gwl *GlobalnameWordlistTrigger) Description() (description string) {
	if gwl.Blacklist {
		return "Triggers when a member joins with a Globalname that contains a word in the specified list"
	}

	return "Triggers when a member joins with a Globalname that contains a words not in the specified list"
}

func (gwl *GlobalnameWordlistTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
		{
			Name:    SanitizeTextName,
			Key:     "SanitizeText",
			Kind:    SettingTypeBool,
			Default: false,
		},
	}
}

func (gwl *GlobalnameWordlistTrigger) CheckGlobalname(t *TriggerContext) (bool, error) {
	dataCast := t.Data.(*GlobalnameWorldlistData)

	list, err := FindFetchGuildList(t.GS.ID, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	fields := strings.Fields(PrepareMessageForWordCheck(t.MS.User.Globalname))
	if dataCast.SanitizeText {
		messageFieldsFixText := strings.Fields(confusables.SanitizeText(PrepareMessageForWordCheck(t.MS.User.Globalname)))
		fields = append(fields, messageFieldsFixText...) // Could be turned into a 1-liner, lmk if I should or not
	}

	for _, mf := range fields {
		contained := false
		for _, w := range list.Content {
			if strings.EqualFold(mf, w) {
				if gwl.Blacklist {
					// contains a blacklisted word, trigger
					return true, nil
				} else {
					contained = true
					break
				}
			}
		}

		if !gwl.Blacklist && !contained {
			// word not whitelisted, trigger
			return true, nil
		}
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ GlobalnameListener = (*GlobalnameInviteTrigger)(nil)

type GlobalnameInviteTrigger struct {
}

func (gv *GlobalnameInviteTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (gv *GlobalnameInviteTrigger) DataType() interface{} {
	return nil
}

func (gv *GlobalnameInviteTrigger) Name() (name string) {
	return "Join Globalname invite"
}

func (gv *GlobalnameInviteTrigger) Description() (description string) {
	return "Triggers when a member joins with a Globalname that contains a server invite"
}

func (gv *GlobalnameInviteTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (gv *GlobalnameInviteTrigger) CheckGlobalname(t *TriggerContext) (bool, error) {
	if common.ContainsInvite(t.MS.User.Globalname, true, true) != nil {
		return true, nil
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ JoinListener = (*MemberJoinTrigger)(nil)

type MemberJoinTrigger struct {
}

func (mj *MemberJoinTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (mj *MemberJoinTrigger) DataType() interface{} {
	return nil
}

func (mj *MemberJoinTrigger) Name() (name string) {
	return "New Member"
}

func (mj *MemberJoinTrigger) Description() (description string) {
	return "Triggers when a new member join"
}

func (mj *MemberJoinTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (mj *MemberJoinTrigger) CheckJoin(t *TriggerContext) (isAffected bool, err error) {
	return true, nil
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*MessageAttachmentTrigger)(nil)

type MessageAttachmentTrigger struct {
	RequiresAttachment bool
}

func (mat *MessageAttachmentTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (mat *MessageAttachmentTrigger) DataType() interface{} {
	return nil
}

func (mat *MessageAttachmentTrigger) Name() string {
	if mat.RequiresAttachment {
		return "Message with attachments"
	}

	return "Message without attachments"
}

func (mat *MessageAttachmentTrigger) Description() string {
	if mat.RequiresAttachment {
		return "Triggers when a message contains an attachment"
	}

	return "Triggers when a message does not contain an attachment"
}

func (mat *MessageAttachmentTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (mat *MessageAttachmentTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	contains := len(m.GetMessageAttachments()) > 0
	if contains && mat.RequiresAttachment {
		return true, nil
	} else if !contains && !mat.RequiresAttachment {
		return true, nil
	}

	return false, nil
}

func (mat *MessageAttachmentTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

var _ MessageTrigger = (*MessageLengthTrigger)(nil)

type MessageLengthTrigger struct {
	Inverted bool
}
type MessageLengthTriggerData struct {
	Length int
}

func (ml *MessageLengthTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (ml *MessageLengthTrigger) DataType() interface{} {
	return &MessageLengthTriggerData{}
}

func (ml *MessageLengthTrigger) Name() (name string) {
	if ml.Inverted {
		return "Message with less than x characters"
	}

	return "Message with more than x characters"
}

func (ml *MessageLengthTrigger) Description() (description string) {
	if ml.Inverted {
		return "Triggers on messages where the content length is lesser than the specified value"
	}

	return "Triggers on messages where the content length is greater than the specified value"
}

func (ml *MessageLengthTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Length",
			Key:  "Length",
			Kind: SettingTypeInt,
		},
	}
}

func (ml *MessageLengthTrigger) CheckMessage(triggerCtx *TriggerContext, cs *dstate.ChannelState, m *discordgo.Message) (bool, error) {
	dataCast := triggerCtx.Data.(*MessageLengthTriggerData)

	if ml.Inverted {
		return utf8.RuneCountInString(m.Content) < dataCast.Length, nil
	}

	return utf8.RuneCountInString(m.Content) > dataCast.Length, nil
}

/////////////////////////////////////////////////////////////

var _ AutomodListener = (*AutomodExecution)(nil)

type AutomodExecution struct {
}
type AutomodExecutionData struct {
	RuleID string
}

func (am *AutomodExecution) Kind() RulePartType {
	return RulePartTrigger
}

func (am *AutomodExecution) DataType() interface{} {
	return &AutomodExecutionData{}
}
func (am *AutomodExecution) Name() (name string) {
	return "Message triggers Discord Automod"
}

func (am *AutomodExecution) Description() (description string) {
	return "Triggers when a message is detected by Discord Automod"
}
func (am *AutomodExecution) UserSettings() []*SettingDef {
	return []*SettingDef{
		{
			Name: "Rule ID (leave blank for all)",
			Key:  "RuleID",
			Kind: SettingTypeString,
		},
	}
}

func (am *AutomodExecution) CheckRuleID(triggerCtx *TriggerContext, ruleID int64) (bool, error) {
	dataCast := triggerCtx.Data.(*AutomodExecutionData)

	if dataCast.RuleID == fmt.Sprint(ruleID) {
		return true, nil
	}

	if dataCast.RuleID == "" {
		return true, nil
	}

	return false, nil
}
