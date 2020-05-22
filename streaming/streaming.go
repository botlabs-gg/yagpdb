package streaming

import (
	"encoding/json"
	"strconv"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Streaming",
		SysName:  "streaming",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
}

type Config struct {
	Enabled bool `json:"enabled" schema:"enabled"` // Wether streaming notifications is enabled or not

	// Give a role to people streaming
	GiveRole int64 `json:"give_role,string" schema:"give_role" valid:"role,true"`
	// Ignores people with this role, requirerole is ignored if this is set
	IgnoreRole int64 `json:"ban_role,string" schema:"ignore_role" valid:"role,true"`
	// Requires people to have this role
	RequireRole int64 `json:"require_role,string" schema:"require_role" valid:"role,true"`

	// Channel to send streaming announcements in
	AnnounceChannel int64 `json:"announce_channel,string" schema:"announce_channel" valid:"channel,true"`
	// The message
	AnnounceMessage string `json:"announce_message" schema:"announce_message" valid:"template,2000"`

	// Match the game name or title against these to filter users out
	GameRegex  string `json:"game_regex" schema:"game_regex" valid:"regex,2000"`
	TitleRegex string `json:"title_regex" schema:"title_regex" valid:"regex,2000"`
}

type LegacyConfig struct {
	Enabled bool `json:"enabled" schema:"enabled"` // Wether streaming notifications is enabled or not

	// Give a role to people streaming
	GiveRole string `json:"give_role" schema:"give_role" valid:"role,true"`
	// Ignores people with this role, requirerole is ignored if this is set
	IgnoreRole string `json:"ban_role" schema:"ignore_role" valid:"role,true"`
	// Requires people to have this role
	RequireRole string `json:"require_role" schema:"require_role" valid:"role,true"`

	// Channel to send streaming announcements in
	AnnounceChannel string `json:"announce_channel" schema:"announce_channel" valid:"channel,true"`
	// The message
	AnnounceMessage string `json:"announce_message" schema:"announce_message" valid:"template,2000"`

	// Match the game name or title against these to filter users out
	GameRegex  string `json:"game_regex" schema:"game_regex" valid:"regex,2000"`
	TitleRegex string `json:"title_regex" schema:"title_regex" valid:"regex,2000"`
}

func (c *Config) UnmarshalJSON(b []byte) error {
	var tmp LegacyConfig
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}

	c.GiveRole, _ = strconv.ParseInt(tmp.GiveRole, 10, 64)
	c.IgnoreRole, _ = strconv.ParseInt(tmp.IgnoreRole, 10, 64)
	c.RequireRole, _ = strconv.ParseInt(tmp.RequireRole, 10, 64)
	c.AnnounceChannel, _ = strconv.ParseInt(tmp.AnnounceChannel, 10, 64)

	c.GameRegex = tmp.GameRegex
	c.TitleRegex = tmp.TitleRegex
	c.Enabled = tmp.Enabled
	c.AnnounceMessage = tmp.AnnounceMessage

	return nil
}

func (c *Config) Save(guildID int64) error {
	return common.SetRedisJson("streaming_config:"+discordgo.StrID(guildID), c)
}

var DefaultConfig = &Config{
	Enabled:         false,
	AnnounceMessage: "OH WOWIE! **{{.User.Username}}** is currently streaming! Check it out: {{.URL}}",
}

// Returns he guild's conifg, or the defaul one if not set
func GetConfig(guildID int64) (*Config, error) {
	var config *Config
	err := common.GetRedisJson("streaming_config:"+discordgo.StrID(guildID), &config)
	if err == nil && config == nil {
		return DefaultConfig, nil
	}

	return config, err
}

var _ featureflags.PluginWithFeatureFlags = (*Plugin)(nil)

const (
	featureFlagEnabled = "streaming_enabled"
)

func (p *Plugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	config, err := GetConfig(guildID)
	if err != nil {
		return nil, errors.WithStackIf(err)
	}

	var flags []string
	if config.Enabled && (config.GiveRole != 0 || config.AnnounceChannel != 0) {
		flags = append(flags, featureFlagEnabled)
	}

	return flags, nil
}

func (p *Plugin) AllFeatureFlags() []string {
	return []string{
		featureFlagEnabled, // set if this server uses the streaming notifications feature
	}
}
