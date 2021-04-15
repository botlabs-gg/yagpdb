package automod

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v2"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/automod_legacy"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/safebrowsing"
)

var forwardSlashReplacer = strings.NewReplacer("\\", "")

/////////////////////////////////////////////////////////////

type BaseRegexTriggerData struct {
	Regex string `valid:",1,250"`
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
		&SettingDef{
			Name: "Regex",
			Key:  "Regex",
			Kind: SettingTypeString,
			Min:  1,
			Max:  250,
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
	return "Triggers when a message includes more than x unique mentions."
}

func (mc *MentionsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Threshold",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 4,
		},
	}
}

func (mc *MentionsTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	dataCast := data.(*MentionsTriggerData)
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

func (alc *AnyLinkTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, stripped string, data interface{}) (bool, error) {
	if common.LinkRegex.MatchString(forwardSlashReplacer.Replace(m.Content)) {
		return true, nil
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
	ListID int64
}

func (wl *WordListTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (wl *WordListTrigger) DataType() interface{} {
	return &WorldListTriggerData{}
}

func (wl *WordListTrigger) Name() (name string) {
	if wl.Blacklist {
		return "Word blacklist"
	}

	return "Word whitelist"
}

func (wl *WordListTrigger) Description() (description string) {
	if wl.Blacklist {
		return "Triggers on messages containing words in the specified list"
	}

	return "Triggers on messages containing words not in the specified list"
}

func (wl *WordListTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
	}
}

func (wl *WordListTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	dataCast := data.(*WorldListTriggerData)

	list, err := FindFetchGuildList(cs.Guild, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	messageFields := strings.Fields(mdStripped)

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
		return "Website blacklist"
	}

	return "Website whitelist"
}

func (dt *DomainTrigger) Description() (description string) {
	if dt.Blacklist {
		return "Triggers on messages containing links to websites in the specified list"
	}

	return "Triggers on messages containing links to websites NOT in the specified list"
}

func (dt *DomainTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
	}
}

func (dt *DomainTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	dataCast := data.(*DomainTriggerData)

	list, err := FindFetchGuildList(cs.Guild, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	matches := common.LinkRegex.FindAllString(forwardSlashReplacer.Replace(m.Content), -1)

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
	return "Triggers when a user has more than x violations within y minutes."
}

func (vt *ViolationsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Violation name",
			Key:     "Name",
			Kind:    SettingTypeString,
			Default: "name",
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
		&SettingDef{
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
	MinLength  int
	Percentage int
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
	return "Triggers when a message contains more than x% of just capitalized letters"
}

func (caps *AllCapsTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Min number of all caps",
			Key:     "MinLength",
			Kind:    SettingTypeInt,
			Default: 3,
		},
		&SettingDef{
			Name:    "Percentage of all caps",
			Key:     "Percentage",
			Kind:    SettingTypeInt,
			Default: 100,
			Min:     1,
			Max:     100,
		},
	}
}

func (caps *AllCapsTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	dataCast := data.(*AllCapsTriggerData)

	if len(m.Content) < dataCast.MinLength {
		return false, nil
	}

	totalCapitalisableChars := 0
	numCaps := 0

	// count the number of upper case characters, note that this dosen't include other characters such as punctuation
	for _, r := range m.Content {
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

func (inv *ServerInviteTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	containsBadInvited := automod_legacy.CheckMessageForBadInvites(m.Content, m.GuildID)
	return containsBadInvited, nil
}

func (inv *ServerInviteTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
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

func (g *GoogleSafeBrowsingTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	threat, err := safebrowsing.CheckString(forwardSlashReplacer.Replace(m.Content))
	if err != nil {
		logger.WithError(err).Error("Failed checking urls against google safebrowser")
		return false, nil
	}

	if threat != nil {
		return true, nil
	}

	return false, nil
}

func (g *GoogleSafeBrowsingTrigger) MergeDuplicates(data []interface{}) interface{} {
	return data[0] // no point in having duplicates of this
}

/////////////////////////////////////////////////////////////

type SlowmodeTriggerData struct {
	Treshold int
	Interval int
}

var _ MessageTrigger = (*SlowmodeTrigger)(nil)

type SlowmodeTrigger struct {
	ChannelBased bool
	Attachments  bool // whether this trigger checks any messages or just attachments
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

		return "x channel messages in y seconds"
	}

	if s.Attachments {
		return "x user attachments in y seconds"
	}
	return "x user messages in y seconds"
}

func (s *SlowmodeTrigger) Description() string {
	if s.ChannelBased {
		if s.Attachments {
			return "Triggers when a channel has more than x attachments within y seconds"
		}

		return "Triggers when a channel has more than x messages in y seconds."
	}

	if s.Attachments {
		return "Triggers when a user has more than x attachments within y seconds in a single channel"
	}

	return "Triggers when a user has more than x messages in y seconds in a single channel."
}

func (s *SlowmodeTrigger) UserSettings() []*SettingDef {
	defaultMessages := 5
	defaultInterval := 5

	if s.Attachments {
		defaultMessages = 10
		defaultInterval = 60
	}

	return []*SettingDef{
		&SettingDef{
			Name:    "Messages",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: defaultMessages,
		},
		&SettingDef{
			Name:    "Within (seconds)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: defaultInterval,
		},
	}
}

func (s *SlowmodeTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	if s.Attachments && len(m.Attachments) < 1 {
		return false, nil
	}

	settings := data.(*SlowmodeTriggerData)

	within := time.Duration(settings.Interval) * time.Second
	now := time.Now()

	amount := 1

	cs.Owner.RLock()
	defer cs.Owner.RUnlock()

	// New messages are at the end
	for i := len(cs.Messages) - 1; i >= 0; i-- {
		cMsg := cs.Messages[i]

		age := now.Sub(cMsg.ParsedCreated)
		if age > within {
			break
		}

		if m.ID == cMsg.ID {
			continue
		}

		if !s.ChannelBased && cMsg.Author.ID != ms.ID {
			continue
		}

		if s.Attachments && len(cMsg.Attachments) < 1 {
			continue // were only checking messages with attachments
		}

		amount++
	}

	if amount >= settings.Treshold {
		return true, nil
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
		return "Triggers when a channel has more than x unique mentions in y seconds"
	}

	return "Triggers when a user has sent more than x unique mentions in y seconds in a single channel"
}

func (mt *MultiMsgMentionTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name:    "Mentions",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Default: 20,
		},
		&SettingDef{
			Name:    "Within (seconds)",
			Key:     "Interval",
			Kind:    SettingTypeInt,
			Default: 10,
		},
		&SettingDef{
			Name: "Count multiple mentions to the same user",
			Key:  "CountDuplicates",
			Kind: SettingTypeBool,
		},
	}
}

func (mt *MultiMsgMentionTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	if len(m.Mentions) < 1 {
		return false, nil
	}

	settings := data.(*MultiMsgMentionTriggerData)

	within := time.Duration(settings.Interval) * time.Second
	now := time.Now()

	mentions := make([]int64, 0)

	cs.Owner.RLock()
	defer cs.Owner.RUnlock()
	// New messages are at the end
	for i := len(cs.Messages) - 1; i >= 0; i-- {
		cMsg := cs.Messages[i]

		age := now.Sub(cMsg.ParsedCreated)
		if age > within {
			break
		}

		if mt.ChannelBased || cMsg.Author.ID == ms.ID {
			// we only care about unique mentions, e.g mentioning the same user a ton wont do anythin
			for _, msgMention := range cMsg.Mentions {
				if msgMention == nil {
					continue
				}

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

func (r *MessageRegexTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	dataCast := data.(*BaseRegexTriggerData)

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
	if re.MatchString(m.Content) {
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

type SpamTriggerData struct {
	Treshold  int
	TimeLimit int
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
		&SettingDef{
			Name:    "Threshold",
			Key:     "Treshold",
			Kind:    SettingTypeInt,
			Min:     1,
			Max:     250,
			Default: 4,
		},
		&SettingDef{
			Name:    "Within seconds (0 = infinity)",
			Key:     "TimeLimit",
			Kind:    SettingTypeInt,
			Min:     0,
			Max:     10000,
			Default: 30,
		},
	}
}

func (spam *SpamTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {

	settingsCast := data.(*SpamTriggerData)

	mToCheckAgainst := strings.TrimSpace(strings.ToLower(m.Content))

	count := 1

	timeLimit := time.Now().Add(-time.Second * time.Duration(settingsCast.TimeLimit))

	cs.Owner.RLock()
	for i := len(cs.Messages) - 1; i >= 0; i-- {
		cMsg := cs.Messages[i]

		if cMsg.ID == m.ID {
			continue
		}

		if cMsg.Author.ID != m.Author.ID {
			continue
		}

		if settingsCast.TimeLimit > 0 && timeLimit.After(cMsg.ParsedCreated) {
			// if this message was created before the time limit, then break out
			break
		}

		if len(cMsg.Attachments) > 0 {
			break // treat any attachment as a different message, in the future i may download them and check hash or something? maybe too much
		}

		if strings.ToLower(strings.TrimSpace(cMsg.Content)) == mToCheckAgainst {
			count++
		} else {
			break
		}
	}

	cs.Owner.RUnlock()

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

func (r *NicknameRegexTrigger) CheckNickname(ms *dstate.MemberState, data interface{}) (bool, error) {
	dataCast := data.(*BaseRegexTriggerData)

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
	if re.MatchString(ms.Nick) {
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
	ListID int64
}

func (nwl *NicknameWordlistTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (nwl *NicknameWordlistTrigger) DataType() interface{} {
	return &NicknameWordlistTriggerData{}
}

func (nwl *NicknameWordlistTrigger) Name() (name string) {
	if nwl.Blacklist {
		return "Nickname word blacklist"
	}

	return "Nickname word whitelist"
}

func (nwl *NicknameWordlistTrigger) Description() (description string) {
	if nwl.Blacklist {
		return "Triggers when a member has a nickname containing words in the specified list, this is currently very easy to circumvent atm, and will likely be improved in the future."
	}

	return "Triggers when a member has a nickname containing words not in the specified list, this is currently very easy to circumvent atm, and will likely be improved in the future."
}

func (nwl *NicknameWordlistTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
	}
}

func (nwl *NicknameWordlistTrigger) CheckNickname(ms *dstate.MemberState, data interface{}) (bool, error) {
	dataCast := data.(*NicknameWordlistTriggerData)

	list, err := FindFetchGuildList(ms.Guild, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	fields := strings.Fields(PrepareMessageForWordCheck(ms.Nick))

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

var _ UsernameListener = (*UsernameRegexTrigger)(nil)

type UsernameRegexTrigger struct {
	BaseRegexTrigger
}

func (r *UsernameRegexTrigger) Name() string {
	if r.BaseRegexTrigger.Inverse {
		return "Join username not matching regex"
	}

	return "Join username matches regex"
}

func (r *UsernameRegexTrigger) Description() string {
	if r.BaseRegexTrigger.Inverse {
		return "Triggers when a member joins with a username that does not match the provided regex"
	}

	return "Triggers when a member joins with a username that matches the provided regex"
}

func (r *UsernameRegexTrigger) CheckUsername(ms *dstate.MemberState, data interface{}) (bool, error) {
	dataCast := data.(*BaseRegexTriggerData)

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
	if re.MatchString(ms.Username) {
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

var _ UsernameListener = (*UsernameWordlistTrigger)(nil)

type UsernameWordlistTrigger struct {
	Blacklist bool
}
type UsernameWorldlistData struct {
	ListID int64
}

func (uwl *UsernameWordlistTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (uwl *UsernameWordlistTrigger) DataType() interface{} {
	return &UsernameWorldlistData{}
}

func (uwl *UsernameWordlistTrigger) Name() (name string) {
	if uwl.Blacklist {
		return "Join username word blacklist"
	}

	return "Join username word whitelist"
}

func (uwl *UsernameWordlistTrigger) Description() (description string) {
	if uwl.Blacklist {
		return "Triggers when a member joins with a username that contains a word in the specified list"
	}

	return "Triggers when a member joins with a username that contains a words not in the specified list"
}

func (uwl *UsernameWordlistTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{
		&SettingDef{
			Name: "List",
			Key:  "ListID",
			Kind: SettingTypeList,
		},
	}
}

func (uwl *UsernameWordlistTrigger) CheckUsername(ms *dstate.MemberState, data interface{}) (bool, error) {
	dataCast := data.(*UsernameWorldlistData)

	list, err := FindFetchGuildList(ms.Guild, dataCast.ListID)
	if err != nil {
		return false, nil
	}

	fields := strings.Fields(PrepareMessageForWordCheck(ms.Username))

	for _, mf := range fields {
		contained := false
		for _, w := range list.Content {
			if strings.EqualFold(mf, w) {
				if uwl.Blacklist {
					// contains a blacklisted word, trigger
					return true, nil
				} else {
					contained = true
					break
				}
			}
		}

		if !uwl.Blacklist && !contained {
			// word not whitelisted, trigger
			return true, nil
		}
	}

	return false, nil
}

/////////////////////////////////////////////////////////////

var _ UsernameListener = (*UsernameInviteTrigger)(nil)

type UsernameInviteTrigger struct {
}

func (uv *UsernameInviteTrigger) Kind() RulePartType {
	return RulePartTrigger
}

func (uv *UsernameInviteTrigger) DataType() interface{} {
	return nil
}

func (uv *UsernameInviteTrigger) Name() (name string) {
	return "Join username invite"
}

func (uv *UsernameInviteTrigger) Description() (description string) {
	return "Triggers when a member joins with a username that contains a server invite"
}

func (uv *UsernameInviteTrigger) UserSettings() []*SettingDef {
	return []*SettingDef{}
}

func (uv *UsernameInviteTrigger) CheckUsername(ms *dstate.MemberState, data interface{}) (bool, error) {
	if common.ContainsInvite(ms.Username, true, true) != nil {
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

func (mj *MemberJoinTrigger) CheckJoin(ms *dstate.MemberState, data interface{}) (isAffected bool, err error) {
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

func (mat *MessageAttachmentTrigger) CheckMessage(ms *dstate.MemberState, cs *dstate.ChannelState, m *discordgo.Message, mdStripped string, data interface{}) (bool, error) {
	contains := len(m.Attachments) > 0
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
