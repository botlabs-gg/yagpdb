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
