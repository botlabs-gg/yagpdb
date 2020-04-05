package automod_legacy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/safebrowsing"
	"github.com/mediocregopher/radix/v3"
)

var forwardSlashReplacer = strings.NewReplacer("\\", "")

type Punishment int

const (
	PunishNone Punishment = iota
	PunishMute
	PunishKick
	PunishBan
)

type Rule interface {
	Check(m *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error)
	ShouldIgnore(msg *discordgo.Message, m *dstate.MemberState) bool
	GetMuteDuration() int
}

type BaseRule struct {
	Enabled bool
	// Name    string

	ViolationsExpire int `valid:"0,44640"`

	// Execute these punishments after certain number of repeated violaions
	MuteAfter    int `valid:"0,1000"`
	MuteDuration int `valid:"0,44640"`
	KickAfter    int `valid:"0,1000"`
	BanAfter     int `valid:"0,1000"`

	IgnoreRole     string   `valid:"role,true"`
	IgnoreChannels []string `valid:"channel,false"`
}

func (r BaseRule) GetMuteDuration() int {
	return r.MuteDuration
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

	common.RedisPool.Do(radix.FlatCmd(nil, "EXPIRE", key, r.ViolationsExpire))

	mute := r.MuteAfter > 0 && violations >= r.MuteAfter
	kick := r.KickAfter > 0 && violations >= r.KickAfter
	ban := r.BanAfter > 0 && violations >= r.BanAfter

	if ban {
		p = PunishBan
	} else if kick {
		p = PunishKick
	} else if mute {
		p = PunishMute
	}

	return
}

// Returns true if this rule should be ignored
func (r BaseRule) ShouldIgnore(evt *discordgo.Message, ms *dstate.MemberState) bool {
	if !r.Enabled {
		return true
	}

	strC := discordgo.StrID(evt.ChannelID)
	for _, ignoreChannel := range r.IgnoreChannels {
		if ignoreChannel == strC {
			return true
		}
	}

	for _, role := range ms.Roles {
		if r.IgnoreRoleInt() == role {
			return true
		}
	}

	return false
}

type SpamRule struct {
	BaseRule `valid:"traverse"`

	NumMessages int `valid:"0,1000"`
	Within      int `valid:"0,100"`
}

// Triggers when a certain number of messages is found by the same author within a timeframe
func (s *SpamRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {

	within := time.Duration(s.Within) * time.Second
	now := time.Now()

	amount := 1

	for i := len(cs.Messages) - 1; i >= 0; i-- {
		cMsg := cs.Messages[i]

		age := now.Sub(cMsg.ParsedCreated)
		if age > within {
			break
		}

		if cMsg.Author.ID == evt.Author.ID && evt.ID != cMsg.ID {
			amount++
		}
	}

	if amount < s.NumMessages {
		return
	}

	del = true

	punishment, err = s.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "spam"))
	if err != nil {
		return
	}

	msg = "Sending messages too fast."

	return
}

type InviteRule struct {
	BaseRule `valid:"traverse"`
}

func (i *InviteRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if !CheckMessageForBadInvites(evt.ContentWithMentionsReplaced(), cs.Guild.ID) {
		return
	}

	del = true

	punishment, err = i.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "invite"))
	if err != nil {
		return
	}

	msg = "Sending server invites to another server."
	return
}

func CheckMessageForBadInvites(msg string, guildID int64) (containsBadInvites bool) {
	// check third party sites
	if common.ContainsInvite(msg, false, true) != nil {
		return true
	}

	matches := common.DiscordInviteSource.Regex.FindAllStringSubmatch(msg, -1)
	if len(matches) < 1 {
		return false
	}

	// Only check each invite id once
	checked := make([]string, 0)

OUTER:
	for _, v := range matches {
		if len(v) < 3 {
			continue
		}

		id := v[2]

		// only check each link once
		for _, c := range checked {
			if id == c {
				continue OUTER
			}
		}

		checked = append(checked, id)

		// Check to see if its a valid id, and if so check if its to the same server were on
		invite, err := common.BotSession.Invite(id)
		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("Failed checking invite ", invite)
			return true // assume bad since discord...
		}

		if invite == nil || invite.Guild == nil {
			continue
		}

		// Ignore invites to this server
		if invite.Guild.ID == guildID {
			continue
		}

		return true
	}

	// If we got here then there's no bad invites
	return false
}

type MentionRule struct {
	BaseRule `valid:"traverse"`

	Treshold int `valid:"0,500"`
}

func (m *MentionRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {
	if len(evt.Mentions) < m.Treshold {
		return
	}

	del = true

	punishment, err = m.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "mention"))
	if err != nil {
		return
	}
	msg = "Sending too many mentions."
	return
}

type LinksRule struct {
	BaseRule `valid:"traverse"`
}

func (l *LinksRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {

	if !common.LinkRegex.MatchString(forwardSlashReplacer.Replace(evt.Content)) {
		return
	}

	del = true
	punishment, err = l.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "links"))
	if err != nil {
		return
	}

	msg = "You do not have permission to send links"

	return
}

type WordsRule struct {
	BaseRule          `valid:"traverse"`
	BuiltinSwearWords bool
	BannedWords       string          `valid:",25000"`
	compiledWords     map[string]bool `json:"-"`
}

func (w *WordsRule) GetCompiled() map[string]bool {
	if w.compiledWords != nil {
		return w.compiledWords
	}

	w.compiledWords = make(map[string]bool)
	fields := strings.Fields(w.BannedWords)
	for _, word := range fields {
		w.compiledWords[strings.ToLower(word)] = true
	}

	return w.compiledWords
}

func (w *WordsRule) Check(evt *discordgo.Message, cs *dstate.ChannelState) (del bool, punishment Punishment, msg string, err error) {

	word := w.CheckMessage(evt.Content)
	if word == "" {
		return
	}

	// Fonud a bad word!
	del = true
	punishment, err = w.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "badword"))

	msg = fmt.Sprintf("The word `%s` is banned, watch your language.", word)
	return
}

func (w *WordsRule) CheckMessage(content string) (word string) {
	userBanned := w.GetCompiled()

	lower := strings.ToLower(content)
	messageWords := strings.Fields(lower)
	for _, v := range messageWords {
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

	BuiltinBadSites           bool
	GoogleSafeBrowsingEnabled bool

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
	banned, item, threatList := s.checkMessage(forwardSlashReplacer.Replace(evt.Content))
	if !banned {
		return
	}

	punishment, err = s.PushViolation(KeyViolations(cs.Guild.ID, evt.Author.ID, "badlink"))
	extraInfo := ""
	if threatList != "" {
		extraInfo = "(sb: " + threatList + ")"
	}

	msg = fmt.Sprintf("The website `%s` is banned %s", item, extraInfo)
	del = true
	return
}

func (s *SitesRule) checkMessage(message string) (banned bool, item string, threatList string) {
	matches := common.LinkRegex.FindAllString(message, -1)

	for _, v := range matches {

		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "steam://") {
			v = "http://" + v
		}

		parsed, err := url.ParseRequestURI(v)
		if err != nil {
			logger.WithError(err).WithField("url", v).Error("Failed parsing request url matched with regex")
		} else {
			if banned, item := s.isBanned(parsed.Host); banned {
				return true, item, ""
			}
		}

	}

	// Check safebrowsing
	if !s.GoogleSafeBrowsingEnabled {
		return false, "", ""
	}

	threat, err := safebrowsing.CheckString(message)
	if err != nil {
		logger.WithError(err).Error("Failed checking urls against google safebrowser")
		return false, "", ""
	}

	if threat != nil {
		return true, threat.Pattern, threat.ThreatType.String()
	}

	return false, "", ""
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
