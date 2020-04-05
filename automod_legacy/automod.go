package automod_legacy

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

var logger = common.GetPluginLogger(&Plugin{})

type Condition string

// Redis keys
func KeyEnabled(gID int64) string { return "automod_enabled:" + discordgo.StrID(gID) }
func KeyConfig(gID int64) string  { return "automod_config:" + discordgo.StrID(gID) }

func KeyViolations(gID, uID int64, violation string) string {
	return "automod_words_violations_" + violation + ":" + discordgo.StrID(gID) + ":" + discordgo.StrID(uID)
}

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Basic Automod",
		SysName:  "legacy_automod",
		Category: common.PluginCategoryModeration,
	}
}

func (p *Plugin) Name() string    { return "Basic Automod" }
func (p *Plugin) SysName() string { return "legacy_automod" }

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

func GetConfig(guildID int64) (config *Config, err error) {

	config = NewConfig()
	err = common.GetRedisJson(KeyConfig(guildID), &config)
	return
}

var (
	_ web.SimpleConfigSaver = (*Config)(nil)
)

func (c Config) Save(guildID int64) error {
	return common.SetRedisJson(KeyConfig(guildID), c)
}
