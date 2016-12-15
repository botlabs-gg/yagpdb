package automod

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Punishment int

const (
	PunishNone Punishment = iota
	PunishMute
	PunishKick
	PunishBan
)

type Rule interface {
	Check(m *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error)
	ShouldIgnore(msg *discordgo.Message, m *discordgo.Member) bool
	GetMuteDuration() int
}

type BaseRule struct {
	Enabled bool
	// Name    string

	ViolationsExpire int `valid:"0,1440"`

	// Execute these punishments after certain number of repeated violaions
	MuteAfter    int `valid:"0,100"`
	MuteDuration int `valid:"0,100"`
	KickAfter    int `valid:"0,100"`
	BanAfter     int `valid:"0,100"`

	IgnoreRole     string   `valid:"role,true"`
	IgnoreChannels []string `valid:"channel,false"`
}

func (r BaseRule) GetMuteDuration() int {
	return r.MuteDuration
}

func (r BaseRule) PushViolation(client *redis.Client, key string) (p Punishment, err error) {
	violations := 0
	violations, err = client.Cmd("INCR", key).Int()
	if err != nil {
		return
	}

	client.Cmd("EXPIRE", key, r.ViolationsExpire)

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
func (r BaseRule) ShouldIgnore(evt *discordgo.Message, m *discordgo.Member) bool {
	if !r.Enabled {
		return true
	}

	for _, ignoreChannel := range r.IgnoreChannels {
		if ignoreChannel == evt.ChannelID {
			return true
		}
	}

	for _, role := range m.Roles {
		if r.IgnoreRole == role {
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
func (s *SpamRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {

	within := time.Duration(s.Within) * time.Second
	now := time.Now()

	amount := 1

	for i := len(channel.Messages) - 1; i >= 0; i-- {
		cMsg := channel.Messages[i]

		cMsgParsed, err := cMsg.Timestamp.Parse()
		if err != nil {
			logrus.WithError(err).Error("Error parsing message timestamp")
			continue
		}

		age := now.Sub(cMsgParsed)
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

	punishment, err = s.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "spam"))
	if err != nil {
		return
	}

	msg = "Sending messages too fast."

	return
}

type InviteRule struct {
	BaseRule `valid:"traverse"`
}

var inviteRegex = regexp.MustCompile(`discord\.gg(?:\/#)?(?:\/invite)?\/([a-zA-Z0-9-]+)`)

func (i *InviteRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {
	matches := inviteRegex.FindAllStringSubmatch(evt.ContentWithMentionsReplaced(), -1)
	if len(matches) < 1 {
		return
	}

	checked := make([]string, 0)

	badInvite := false

OUTER:
	for _, v := range matches {
		if len(v) < 2 {
			continue
		}
		id := v[1]

		// only check each link once
		for _, c := range checked {
			if id == c {
				continue OUTER
			}
		}

		checked = append(checked, id)

		invite, err := common.BotSession.Invite(id)
		if err != nil {
			logrus.Error(err)
			continue
		}

		// Ignore invites to this server
		if invite.Guild.ID == channel.GuildID {
			continue
		}

		badInvite = true
		break
	}

	if !badInvite {
		return
	}

	del = true

	punishment, err = i.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "invite"))
	if err != nil {
		return
	}

	msg = "Sending server invites to another server."
	return
}

type MentionRule struct {
	BaseRule `valid:"traverse"`

	Treshold int `valid:"0,500"`
}

func (m *MentionRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {
	if len(evt.Mentions) < m.Treshold {
		return
	}

	del = true

	punishment, err = m.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "mention"))
	if err != nil {
		return
	}
	msg = "Sending too many mentions."
	return
}

type LinksRule struct {
	BaseRule `valid:"traverse"`
}

var linkRegex = regexp.MustCompile(`^((https?|steam):\/\/[^\s<]+[^<.,:;"')\]\s])`)

func (l *LinksRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {

	if !linkRegex.MatchString(evt.ContentWithMentionsReplaced()) {
		return
	}

	del = true
	punishment, err = l.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "links"))
	if err != nil {
		return
	}

	msg = "You do not have permission to send links"

	return
}

type WordsRule struct {
	BaseRule          `valid:"traverse"`
	BuiltinSwearWords bool
	BannedWords       string          `valid:",10000"`
	compiledWords     map[string]bool `json:"-"`
}

func (w *WordsRule) GetCompiled() map[string]bool {
	if w.compiledWords != nil {
		return w.compiledWords
	}

	w.compiledWords = make(map[string]bool)
	fields := strings.Fields(w.BannedWords)
	for _, word := range fields {
		w.compiledWords[word] = true
	}

	return w.compiledWords
}

func (w *WordsRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {

	found := false

	userBanned := w.GetCompiled()

	lower := strings.ToLower(evt.Content)
	messageWords := strings.Fields(lower)
	for _, v := range messageWords {
		if _, ok := userBanned[v]; ok {
			found = true
			break
		}

		if w.BuiltinSwearWords {
			if _, ok := BuiltinSwearWords[v]; ok {
				found = true
				break
			}
		}
	}

	if !found {
		return
	}

	// Fonud a bad word!
	del = true
	punishment, err = w.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "badword"))

	msg = "You triggered the word filter, watch your language."
	return
}

type SitesRule struct {
	BaseRule `valid:"traverse"`

	BuiltinBadSites  bool
	BuiltinPornSites bool

	BannedWebsites   string `valid:",10000"`
	compiledWebsites map[string]bool
}

func (w *SitesRule) GetCompiled() map[string]bool {
	if w.compiledWebsites != nil {
		return w.compiledWebsites
	}

	w.compiledWebsites = make(map[string]bool)
	fields := strings.Fields(w.BannedWebsites)
	for _, field := range fields {
		w.compiledWebsites[field] = true
	}

	return w.compiledWebsites
}

func (s *SitesRule) Check(evt *discordgo.Message, channel *discordgo.Channel, client *redis.Client) (del bool, punishment Punishment, msg string, err error) {
	matches := linkRegex.FindAllString(evt.Content, -1)

	bannedLink := false

	bannedLinks := s.GetCompiled()
	for _, v := range matches {
		parsed, err := url.ParseRequestURI(v)
		if err != nil {
			logrus.WithError(err).WithField("url", v).Error("Failed parsing request url matched with regex")
		} else {
			if _, ok := bannedLinks[strings.ToLower(parsed.Host)]; ok {
				bannedLink = true
				break
			}
		}
	}

	if !bannedLink {
		return
	}

	punishment, err = s.PushViolation(client, KeyViolations(channel.GuildID, evt.Author.ID, "badlink"))
	msg = "That website is banned. Contact an admin if you think this was a mistake."
	del = true
	return
}
