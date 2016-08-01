package moderation

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Moderation"
}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
}

type Config struct {
	BanEnabled    bool
	KickEnabled   bool
	CleanEnabled  bool
	ReportEnabled bool
	ActionChannel string
	ReportChannel string
}

func (c *Config) Save(client *redis.Client, guildID string) error {
	client.Append("SET", "moderation_ban_enabled:"+guildID, c.BanEnabled)
	client.Append("SET", "moderation_kick_enabled:"+guildID, c.KickEnabled)
	client.Append("SET", "moderation_clean_enabled:"+guildID, c.CleanEnabled)
	client.Append("SET", "moderation_report_enabled:"+guildID, c.ReportEnabled)
	client.Append("SET", "moderation_action_channel:"+guildID, c.ActionChannel)
	client.Append("SET", "moderation_report_channel:"+guildID, c.ReportChannel)

	replies := common.GetRedisReplies(client, 6)
	for _, r := range replies {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

func GetConfig(client *redis.Client, guildID string) (config *Config, err error) {
	client.Append("GET", "moderation_ban_enabled:"+guildID)
	client.Append("GET", "moderation_kick_enabled:"+guildID)
	client.Append("GET", "moderation_clean_enabled:"+guildID)
	client.Append("GET", "moderation_report_enabled:"+guildID)
	client.Append("GET", "moderation_action_channel:"+guildID)
	client.Append("GET", "moderation_report_channel:"+guildID)

	replies := common.GetRedisReplies(client, 6)

	for _, r := range replies {
		if r.Err != nil {
			// Check if the error ws caused by the key not existing
			if _, ok := r.Err.(*redis.CmdError); !ok {
				return nil, r.Err
			}
		}
	}

	// We already checked errors above, althoug if someone were to fuck shit up manually
	// Then yeah, these would be default values
	banEnabled, _ := replies[0].Bool()
	kickEnabled, _ := replies[1].Bool()
	cleanEnabled, _ := replies[2].Bool()
	reportEnabled, _ := replies[3].Bool()

	actionChannel, _ := replies[4].Str()
	reportChannel, _ := replies[5].Str()

	return &Config{
		BanEnabled:    banEnabled,
		KickEnabled:   kickEnabled,
		CleanEnabled:  cleanEnabled,
		ReportEnabled: reportEnabled,
		ActionChannel: actionChannel,
		ReportChannel: reportChannel,
	}, nil
}
