package automod_legacy

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/antiphishing"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/safebrowsing"
	"github.com/botlabs-gg/yagpdb/v2/serverpreview"
	gsafebrowsing "github.com/google/safebrowsing"
	"github.com/mediocregopher/radix/v3"
)

var forwardSlashReplacer = strings.NewReplacer("\\", "")

type Punishment int

const (
	PunishNone Punishment = iota
	PunishMute
	PunishTimeout
	PunishKick
	PunishBan
)

type Rule interface {
	Check(m *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error)
	ShouldIgnore(cs *dstate.ChannelState, msg *discordgo.Message, m *dstate.MemberState) bool
	GetMuteDuration() int
	GetTimeoutDuration() time.Duration
}

type BaseRule struct {
	Enabled bool
	// Name    string

	ViolationsExpire int `valid:"0,44640"`

	// Execute these punishments after certain number of repeated violaions
	MuteAfter       int `valid:"0,1000"`
	MuteDuration    int `valid:"0,44640"`
	TimeoutAfter    int `valid:"0,1000"`
	TimeoutDuration int `valid:"0,40320"`
	KickAfter       int `valid:"0,1000"`
	BanAfter        int `valid:"0,1000"`

	IgnoreRole     string   `valid:"role,true"`
	IgnoreChannels []string `valid:"channel,false"`
}

type IncludeExtraTextOption struct {
	IncludeExtraText bool
}

func (o IncludeExtraTextOption) texts(m *discordgo.Message) []string {
	if o.IncludeExtraText {
		return m.GetAllTextContents()
	}
	return []string{m.Content}
}

func messageTexts(m *discordgo.Message, includeExtraText bool) []string {
	if includeExtraText {
		return m.GetAllTextContents()
	}
	return m.GetMessageContents()
}

func (r BaseRule) GetMuteDuration() int {
	return r.MuteDuration
}

func (r BaseRule) GetTimeoutDuration() time.Duration {
	if r.TimeoutDuration < 1 {
		return moderation.DefaultTimeoutDuration
	}
	return time.Duration(r.TimeoutDuration) * time.Minute
}

func (r BaseRule) IgnoreRoleInt() int64 {
	ir, _ := strconv.ParseInt(r.IgnoreRole, 10, 64)
	return ir
}

func (r BaseRule) IgnoreChannelsParsed() []int64 {
	result := make([]int64, 0, len(r.IgnoreChannels))
	for _, str := range r.IgnoreChannels {
		parsed, err := strconv.ParseInt(str, 10, 64)
		if err == nil && parsed != 0 {
			result = append(result, parsed)
		}
	}

	return result
}

func (r BaseRule) PushViolation(key string) (p Punishment, err error) {
	violations := 0
	err = common.RedisPool.Do(radix.Cmd(&violations, "INCR", key))
	if err != nil {
		return
	}

	if r.ViolationsExpire > 0 {
		common.RedisPool.Do(radix.FlatCmd(nil, "EXPIRE", key, r.ViolationsExpire*60))
	}

	p = r.punishmentFor(violations)

	return
}

func (r BaseRule) punishmentFor(violations int) Punishment {
	reached := func(after int) bool { return after > 0 && violations >= after }

	switch {
	case reached(r.BanAfter):
		return PunishBan
	case reached(r.KickAfter):
		return PunishKick
	case reached(r.TimeoutAfter):
		return PunishTimeout
	case reached(r.MuteAfter):
		return PunishMute
	}
	return PunishNone
}

// Returns true if this rule should be ignored
func (r BaseRule) ShouldIgnore(cs *dstate.ChannelState, evt *discordgo.Message, ms *dstate.MemberState) bool {
	if !r.Enabled {
		return true
	}

	strC := discordgo.StrID(common.ChannelOrThreadParentID(cs))
	if slices.Contains(r.IgnoreChannels, strC) {
		return true
	}

	for _, role := range ms.Member.Roles {
		if r.IgnoreRoleInt() == role {
			return true
		}
	}

	return false
}

type SpamRule struct {
	BaseRule `valid:"traverse"`

	NumMessages int `valid:"1,1000"`
	Within      int `valid:"1,100"`
}

// Triggers when a certain number of messages is found by the same author within a timeframe
func (s *SpamRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if !s.FindSpam(evt, cs) {
		return
	}

	del = true

	punishment, err = s.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "spam"))
	if err != nil {
		return
	}

	msg = "Sending messages too fast."

	return
}

func (s *SpamRule) FindSpam(evt *discordgo.Message, cs *dstate.ChannelState) bool {
	if s.Within == 0 {
		s.Within = 1
	}
	within := time.Duration(s.Within) * time.Second
	now := time.Now()

	amount := 1

	messages := bot.State.GetMessages(cs.GuildID, cs.ID, &dstate.MessagesQuery{
		Limit: 1000,
	})

	for _, v := range messages {
		age := now.Sub(v.ParsedCreatedAt)
		if age > within {
			break
		}

		if v.Author.ID == evt.Author.ID && evt.ID != v.ID {
			amount++
		}
	}
	if s.NumMessages < 1 {
		s.NumMessages = 1
	}

	return amount > s.NumMessages
}

type InviteRule struct {
	BaseRule `valid:"traverse"`
	IncludeExtraTextOption
}

func (i *InviteRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if !CheckMessageForBadInvites(evt, i.IncludeExtraText) {
		return
	}

	del = true

	punishment, err = i.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "invite"))
	if err != nil {
		return
	}

	msg = "Sending server invites to another server."
	return
}

type GuildInvites struct {
	createdAt time.Time
	invites   map[string]bool
	failed    bool
}

const failedInviteFetchRetry = 5 * time.Minute

type cachedGuildInvites struct {
	sync.RWMutex
	guilds map[int64]GuildInvites
}

func (c *cachedGuildInvites) gc(d time.Duration) {
	logger.Info("Init worker for invite cache GC")
	ticker := time.NewTicker(d)
	for range ticker.C {
		c.tick(d)
	}
}

func (c *cachedGuildInvites) tick(d time.Duration) {
	logger.Info("Starting invites cache GC")

	t1 := time.Now()
	var counter int

	invitesCache.Lock()
	for guild := range c.guilds {
		if time.Since(c.guilds[guild].createdAt) > d {
			delete(c.guilds, guild)
			counter++
		}
	}

	invitesCache.Unlock()
	logger.Infof("Finished clearing invites cache in %v. %d guilds removed.", time.Since(t1), counter)
}

func (c *cachedGuildInvites) get(guildId int64) (GuildInvites, bool) {
	c.RLock()
	defer c.RUnlock()
	guildInvite, ok := c.guilds[guildId]
	return guildInvite, ok
}

func (c *cachedGuildInvites) set(guildID int64, invites map[string]bool, failed bool) {
	c.Lock()
	defer c.Unlock()
	c.guilds[guildID] = GuildInvites{time.Now(), invites, failed}
}

var invitesCache cachedGuildInvites

func CheckMessageForBadInvites(msg *discordgo.Message, includeExtraText bool) (containsBadInvites bool) {
	guildID := msg.GuildID
	for _, content := range messageTexts(msg, includeExtraText) {
		// check third party sites
		if common.ContainsInvite(content, false, true) != nil {
			return true
		}
		if serverpreview.ContainsOtherServerPreviewLink(content, guildID) {
			return true
		}
		content := confusables.NormalizeQueryEncodedText(content)
		content = confusables.SanitizeText(content)
		matches := common.DiscordInviteSource.Regex.FindAllStringSubmatch(content, -1)
		if len(matches) < 1 {
			continue
		}

		guildInvites, ok := invitesCache.get(guildID)
		if ok && guildInvites.failed && time.Since(guildInvites.createdAt) > failedInviteFetchRetry {
			ok = false
		}
		if !ok { // we do not have a cache for this guild yet, create it
			// failures and empty results are cached too, otherwise every invite-like message
			// in a guild where the bot can't list invites makes another request to discord
			invites, err := common.BotSession.GuildInvites(guildID)
			if err != nil {
				logger.WithError(err).WithField("guild", guildID).Error("Failed fetching invites", invites)
			}

			inviteMap := make(map[string]bool)
			for _, invite := range invites {
				inviteMap[invite.Code] = true
			}
			guild := bot.State.GetGuild(guildID)
			if guild != nil && len(guild.VanityURLCode) > 0 {
				inviteMap[guild.VanityURLCode] = true
			}

			invitesCache.set(guildID, inviteMap, err != nil)

			// assume it is a bad invite if discord failed or the guild has no invites
			if err != nil || len(invites) == 0 {
				return true
			}

			guildInvites = GuildInvites{
				invites: inviteMap,
			}
		}

		// Only check each invite id once,
		// in case it repeats in the message multiple times.
		checked := make(map[string]bool, len(matches))

		for _, v := range matches {
			if len(v) < 3 {
				continue
			}

			id := v[2]
			// only check each link once
			if checked[id] {
				continue
			} else {
				checked[id] = true
			}

			// Safe guard the map look up using the invite mutex.
			// This is a extremely rare, but possible race condition.
			invitesCache.RLock()
			isInviteOk := guildInvites.invites[id]
			invitesCache.RUnlock()

			if isInviteOk {
				// ignore invites to this guild
				continue
			}

			// invite to another guild found
			//
			// PS: we were getting a lot of rate
			// limits issues from discord by
			// validating each individual invite.
			//
			// Thus, we now return true for
			// all possible invites, even if
			// the invite is not valid.
			//
			// Reason being it is much easier
			// to get an actual invite than
			// just a fake one.
			return true
		}
	}

	// If we got here then there are no bad invites
	return false
}

type MentionRule struct {
	BaseRule `valid:"traverse"`

	Treshold int `valid:"1,500"`
}

func (m *MentionRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if m.Treshold == 0 {
		m.Treshold = 1
	}
	if len(evt.Mentions) < m.Treshold {
		return
	}

	del = true

	punishment, err = m.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "mention"))
	if err != nil {
		return
	}
	msg = "Sending too many mentions."
	return
}

type LinksRule struct {
	BaseRule `valid:"traverse"`
	IncludeExtraTextOption
}

func (l *LinksRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if !l.containsLink(evt) {
		return
	}

	del = true
	punishment, err = l.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "links"))
	if err != nil {
		return
	}

	msg = "You do not have permission to send links"

	return
}

func (l *LinksRule) containsLink(evt *discordgo.Message) bool {
	for _, content := range l.texts(evt) {
		content = confusables.NormalizeQueryEncodedText(content)
		if common.LinkRegex.MatchString(forwardSlashReplacer.Replace(content)) {
			return true
		}
	}
	return false
}

type WordsRule struct {
	BaseRule `valid:"traverse"`
	IncludeExtraTextOption
	BuiltinSwearWords bool
	BannedWords       string          `valid:",25000"`
	compiledWords     map[string]bool `json:"-"`
}

func (w *WordsRule) GetCompiled() map[string]bool {
	if w.compiledWords != nil {
		return w.compiledWords
	}

	w.compiledWords = make(map[string]bool)
	fields := strings.FieldsSeq(w.BannedWords)
	for word := range fields {
		w.compiledWords[strings.ToLower(word)] = true
	}

	return w.compiledWords
}

func (w *WordsRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {

	var word string
	for _, content := range w.texts(evt) {
		if word = w.CheckMessage(content); word != "" {
			break
		}
	}
	if word == "" {
		return
	}

	// Fonud a bad word!
	del = true
	punishment, err = w.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "badword"))

	msg = fmt.Sprintf("The word `%s` is banned, watch your language.", word)
	return
}

func (w *WordsRule) CheckMessage(content string) (word string) {
	userBanned := w.GetCompiled()

	lower := strings.ToLower(content)
	messageWords := strings.FieldsSeq(lower)
	for v := range messageWords {
		if _, ok := userBanned[v]; ok {
			return v
		}

		if w.BuiltinSwearWords {
			if _, ok := BuiltinSwearWords[v]; ok {
				return v
			}
		}
	}

	return ""
}

type SitesRule struct {
	BaseRule `valid:"traverse"`
	IncludeExtraTextOption

	GoogleSafeBrowsingEnabled bool
	ScamLinkProtection        bool

	BannedWebsites   string `valid:",10000"`
	compiledWebsites []string
}

func (w *SitesRule) GetCompiled() []string {
	if w.compiledWebsites != nil {
		return w.compiledWebsites
	}

	fields := strings.Fields(w.BannedWebsites)

	w.compiledWebsites = make([]string, len(fields))

	for i, field := range fields {
		w.compiledWebsites[i] = strings.ToLower(field)
	}

	return w.compiledWebsites
}

func (s *SitesRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	banned, item, threatList := s.checkMessage(evt)
	if !banned {
		return
	}

	punishment, err = s.PushViolation(KeyViolations(cs.GuildID, evt.Author.ID, "badlink"))
	extraInfo := ""
	if threatList != "" {
		extraInfo = "(sb: " + threatList + ")"
	}

	msg = fmt.Sprintf("The website `%s` is banned %s", item, extraInfo)
	del = true
	return
}

func (s *SitesRule) checkMessage(message *discordgo.Message) (banned bool, item string, threatList string) {
	// safebrowsing is batched over the contents before the first banned or scam link,
	// so the reported item matches checking each content in order
	var checked []string
	for _, content := range messageTexts(message, s.IncludeExtraText) {
		content = common.ForwardSlashReplacer.Replace(confusables.NormalizeQueryEncodedText(content))
		if item := s.findBannedLink(content); item != "" {
			if threat := s.checkSafeBrowsing(checked); threat != nil {
				return true, threat.Pattern, threat.ThreatType.String()
			}
			return true, item, ""
		}
		checked = append(checked, content)
	}

	if threat := s.checkSafeBrowsing(checked); threat != nil {
		return true, threat.Pattern, threat.ThreatType.String()
	}

	return false, "", ""
}

func (s *SitesRule) findBannedLink(content string) string {
	matches := common.LinkRegex.FindAllString(content, -1)
	for _, v := range matches {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "steam://") {
			v = "http://" + v
		}

		parsed, err := url.ParseRequestURI(v)
		if err != nil {
			logger.WithError(err).WithField("url", v).Error("Failed parsing request url matched with regex")
		} else {
			if banned, item := s.isBanned(parsed.Host); banned {
				return item
			}
		}
	}

	if s.ScamLinkProtection {
		scamLink, err := antiphishing.CheckMessageForPhishingDomains(content)
		if err != nil {
			logger.WithError(err).Error("Failed checking urls against antiphishing APIs")
		} else if scamLink != "" {
			return scamLink
		}
	}

	return ""
}

func (s *SitesRule) checkSafeBrowsing(contents []string) *gsafebrowsing.URLThreat {
	if !s.GoogleSafeBrowsingEnabled || len(contents) == 0 {
		return nil
	}

	threat, err := safebrowsing.CheckStrings(contents)
	if err != nil {
		logger.WithError(err).Error("Failed checking urls against google safebrowser")
	}
	return threat
}

func (s *SitesRule) isBanned(host string) (bool, string) {
	if index := strings.Index(host, ":"); index > -1 {
		host = host[:index]
	}

	host = strings.ToLower(host)

	for _, v := range s.compiledWebsites {
		if s.matchesItem(v, host) {
			return true, v
		}
	}

	return false, ""
}

func (s *SitesRule) matchesItem(filter, str string) bool {

	if strings.HasSuffix(str, "."+filter) {
		return true
	}

	return str == filter
}
