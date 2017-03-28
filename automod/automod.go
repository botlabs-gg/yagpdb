package automod

//go:generate esc -o assets_gen.go -pkg automod -ignore ".go" assets/

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/docs"
	"github.com/jonas747/yagpdb/web"
)

type Condition string

// Redis keys
func KeyEnabled(gID string) string { return "automod_enabled:" + gID }
func KeyConfig(gID string) string  { return "automod_config:" + gID }

func KeyViolations(gID, uID, violation string) string {
	return "automod_words_violations_" + violation + ":" + gID + ":" + uID
}

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}

	web.RegisterPlugin(p)
	bot.RegisterPlugin(p)
	docs.AddPage("Automoderator", FSMustString(false, "/assets/help-page.md"), nil)
}

func (p *Plugin) Name() string { return "Automod" }

type Config struct {
	Enabled bool
	Spam    *SpamRule    `valid:"traverse"`
	Mention *MentionRule `valid:"traverse"`
	Invite  *InviteRule  `valid:"traverse"`
	Links   *LinksRule   `valid:"traverse"`
	Sites   *SitesRule   `valid:"traverse"`
	Words   *WordsRule   `valid:"traverse"`
}

func (c Config) Name() string {
	return "Automoderator"
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
