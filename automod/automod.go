package automod

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Condition string

var (
	KeyEnabled = func(gid string) string { return "automod_enabled:" + gid }

	KeySpam    = func(gid string) string { return "automod_spam:" + gid }
	KeyMention = func(gid string) string { return "automod_mention:" + gid }
	KeyInvite  = func(gid string) string { return "automod_invite:" + gid }
	KeySites   = func(gid string) string { return "automod_sites:" + gid }
	KeyWords   = func(gid string) string { return "automod_words:" + gid }
)

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}

	web.RegisterPlugin(p)
}

func (p *Plugin) Name() string { return "Automod" }

func (p *Plugin) RedisConfigKeys(gID string) []string {
	return []string{
		KeyEnabled(gID),
		KeySpam(gID),
		KeyMention(gID),
		KeyInvite(gID),
		KeySites(gID),
		KeyWords(gID),
	}
}

type BaseRule struct {
	Enabled bool

	ViolationsExpire int

	// Execute these punishments after certain number of repeated violaions
	WarnAfter int
	KickAfter int
	BanAfter  int

	IgnoreRole     string
	IgnoreChannels []string
}

type SpamRule struct {
	BaseRule

	NumMessages int
	Within      int
}

func (s SpamRule) Save(gID string, client *redis.Client) error {
	return common.SetRedisJson(client, KeySpam(gID), s)
}

type MentionRule struct {
	BaseRule

	Treshold int
}

func (m MentionRule) Save(gID string, client *redis.Client) error {
	return common.SetRedisJson(client, KeyMention(gID), m)
}

type InviteRule struct {
	BaseRule
}

func (i InviteRule) Save(gID string, client *redis.Client) error {
	return common.SetRedisJson(client, KeyInvite(gID), i)
}

// Behold the almighty number of return values on this one!
func GetRules(guildID string, client *redis.Client) (spam *SpamRule, mention *MentionRule, invite *InviteRule, words *WordsRule, sites *SitesRule, err error) {

	spam = &SpamRule{}
	invite = &InviteRule{}
	mention = &MentionRule{}
	words = &WordsRule{}
	sites = &SitesRule{BannedWebsites: "somesite.com\nanothersite.org"}

	if err = common.GetRedisJson(client, KeySpam(guildID), spam); err != nil {
		return
	}
	if err = common.GetRedisJson(client, KeyMention(guildID), mention); err != nil {
		return
	}
	if err = common.GetRedisJson(client, KeyInvite(guildID), invite); err != nil {
		return
	}
	if err = common.GetRedisJson(client, KeyWords(guildID), words); err != nil {
		return
	}
	if err = common.GetRedisJson(client, KeySites(guildID), sites); err != nil {
		return
	}

	return
}

func GetEnabled(guildID string, client *redis.Client) (bool, error) {
	reply := client.Cmd("GET", KeyEnabled(guildID))
	if reply.Type == redis.NilReply {
		return false, nil
	}

	return reply.Bool()
}

type WordsRule struct {
	BaseRule
	BuiltinSwearWords bool
	BannedWords       string
}

func (l WordsRule) Save(gID string, client *redis.Client) error {
	return common.SetRedisJson(client, KeyWords(gID), l)
}

type SitesRule struct {
	BaseRule

	BuiltinBadSites  bool
	BuiltinPornSites bool

	BannedWebsites string
}

func (l SitesRule) Save(gID string, client *redis.Client) error {
	return common.SetRedisJson(client, KeySites(gID), l)
}
