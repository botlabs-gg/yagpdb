package streaming

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/web"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Streaming"
}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
}

type Config struct {
	Enabled bool `json:"enabled"` // Wether streaming notifications is enabled or not

	GiveRole string `json:"give_role"` // Give a role to people streaming

	IgnoreRole  string `json:"ban_role"`     // Ignores people with this role, requirerole is ignored if this is set
	RequireRole string `json:"require_role"` // Requires people to have this role

	AnnounceChannel string `json:"announce_channel"` // Channel to send streaming announcements in
	AnnounceMessage string `json:"announce_message"` // The message
}

func (c *Config) Save(client *redis.Client, guildID string) error {
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return client.Cmd("SET", "streaming_config:"+guildID, encoded).Err
}

var DefaultConfig = &Config{
	Enabled:         false,
	AnnounceMessage: "OH WOWIE! **{{.User.Username}}** is currently streaming! Check it out: {{.URL}}",
}

// Returns he guild's conifg, or the defaul one if not set
func GetConfig(client *redis.Client, guildID string) (*Config, error) {
	reply := client.Cmd("GET", "streaming_config:"+guildID)
	if reply.Type == redis.NilReply {
		return DefaultConfig, nil
	}

	b, err := reply.Bytes()
	if err != nil {
		return nil, err
	}

	var config *Config
	err = json.Unmarshal(b, &config)
	return config, err
}
