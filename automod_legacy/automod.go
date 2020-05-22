package automod_legacy

import (
	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
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

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagEnabled = "automod_legacy_enabled"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	config, err := GetConfig(guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	if config.Enabled {
		if config.Spam.Enabled ||
			config.Mention.Enabled ||
			config.Invite.Enabled ||
			config.Links.Enabled ||
			config.Sites.Enabled ||
			config.Words.Enabled {

			// check if atleast one rule is enabled

			flags = append(flags, featureFlagEnabled)
		}
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagEnabled, // set if automod is enabled and atleast one rule is enabled as well
	}
}
