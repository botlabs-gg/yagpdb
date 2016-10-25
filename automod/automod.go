package automod

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Condition string

var (

	// Redis keys
	KeyEnabled = func(gID string) string { return "automod_enabled:" + gID }
	KeyConfig  = func(gID string) string { return "automod_config:" + gID }

	KeyViolations = func(gID, uID, violation string) string {
		return "automod_words_violations_" + violation + ":" + gID + ":" + uID
	}

	// Local Bot Cache keys
	KeyAllRules = func(gID string) string { return "automod_rules:" + gID }
)

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}

	web.RegisterPlugin(p)
	bot.RegisterPlugin(p)
}

func (p *Plugin) Name() string { return "Automod" }

type Config struct {
	Enabled bool
	Spam    *SpamRule
	Mention *MentionRule
	Invite  *InviteRule
	Links   *LinksRule
	Sites   *SitesRule
	Words   *WordsRule
}

func NewConfig() *Config {
	return &Config{
		Spam:    &SpamRule{},
		Mention: &MentionRule{},
		Invite:  &InviteRule{},
		Links:   &LinksRule{},
		Sites:   &SitesRule{},
		Words:   &WordsRule{},
	}
}

// Behold the almighty number of return values on this one!
func GetConfig(client *redis.Client, guildID string) (config *Config, err error) {

	config = NewConfig()
	err = common.GetRedisJson(client, KeyConfig(guildID), &config)
	return
}

func (c Config) Save(client *redis.Client, guildID string) error {
	if err := common.SetRedisJson(client, KeyConfig(guildID), c); err != nil {
		return err
	}
	return nil
}
