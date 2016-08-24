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
	BanMessage    string
	KickMessage   string
}

func (c *Config) Save(client *redis.Client, guildID string) error {
	client.Append("SET", "moderation_ban_enabled:"+guildID, c.BanEnabled)
	client.Append("SET", "moderation_kick_enabled:"+guildID, c.KickEnabled)
	client.Append("SET", "moderation_clean_enabled:"+guildID, c.CleanEnabled)
	client.Append("SET", "moderation_report_enabled:"+guildID, c.ReportEnabled)
	client.Append("SET", "moderation_action_channel:"+guildID, c.ActionChannel)
	client.Append("SET", "moderation_report_channel:"+guildID, c.ReportChannel)
	client.Append("SET", "moderation_ban_message:"+guildID, c.BanMessage)
	client.Append("SET", "moderation_kick_message:"+guildID, c.KickMessage)

	_, err := common.GetRedisReplies(client, 8)
	return err
}

func GetConfig(client *redis.Client, guildID string) (config *Config, err error) {
	client.Append("GET", "moderation_ban_enabled:"+guildID)
	client.Append("GET", "moderation_kick_enabled:"+guildID)
	client.Append("GET", "moderation_clean_enabled:"+guildID)
	client.Append("GET", "moderation_report_enabled:"+guildID)
	client.Append("GET", "moderation_action_channel:"+guildID)
	client.Append("GET", "moderation_report_channel:"+guildID)
	client.Append("GET", "moderation_ban_message:"+guildID)
	client.Append("GET", "moderation_kick_message:"+guildID)

	replies, err := common.GetRedisReplies(client, 8)
	if err != nil {
		return nil, err
	}

	// We already checked errors above, altthough if someone were to fuck shit up manually
	// Then yeah, these would be default values
	banEnabled, _ := replies[0].Bool()
	kickEnabled, _ := replies[1].Bool()
	cleanEnabled, _ := replies[2].Bool()
	reportEnabled, _ := replies[3].Bool()

	actionChannel, _ := replies[4].Str()
	reportChannel, _ := replies[5].Str()

	banMsg, _ := replies[6].Str()
	kickMsg, _ := replies[7].Str()

	return &Config{
		BanEnabled:    banEnabled,
		KickEnabled:   kickEnabled,
		CleanEnabled:  cleanEnabled,
		ReportEnabled: reportEnabled,
		ActionChannel: actionChannel,
		ReportChannel: reportChannel,
		BanMessage:    banMsg,
		KickMessage:   kickMsg,
	}, nil
}
