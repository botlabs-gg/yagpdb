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

/*
	BanEnabled:           r.FormValue("ban_enabled") == "on",
	KickEnabled:          r.FormValue("kick_enabled") == "on",
	ReportEnabled:        r.FormValue("report_enabled") == "on",
	CleanEnabled:         r.FormValue("clean_enabled") == "on",
	DeleteMessagesOnKick: r.FormValue("kick_delete_messages") == "on",
	KickMessage:          r.FormValue("kick_message"),
	BanMessage:           r.FormValue("ban_message"),
*/

type Config struct {
	BanEnabled           bool   `schema:"ban_enabled"`
	KickEnabled          bool   `schema:"kick_enabled"`
	CleanEnabled         bool   `schema:"clean_enabled"`
	ReportEnabled        bool   `schema:"report_enabled"`
	DeleteMessagesOnKick bool   `schema:"kick_delete_messages"`
	ActionChannel        string `schema:"action_channel" valid:"channel,true"`
	ReportChannel        string `schema:"report_channel" valid:"channel,true"`
	BanMessage           string `schema:"kick_message" valid:"template,2000"`
	KickMessage          string `schema:"ban_message" valid:"template,2000"`
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
	client.Append("SET", "moderation_kick_delete_messages:"+guildID, c.DeleteMessagesOnKick)

	_, err := common.GetRedisReplies(client, 9)
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
	client.Append("GET", "moderation_kick_delete_messages:"+guildID)

	replies, err := common.GetRedisReplies(client, 9)
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

	kickDeleteMessages, _ := replies[8].Bool()

	return &Config{
		BanEnabled:           banEnabled,
		KickEnabled:          kickEnabled,
		CleanEnabled:         cleanEnabled,
		ReportEnabled:        reportEnabled,
		ActionChannel:        actionChannel,
		ReportChannel:        reportChannel,
		BanMessage:           banMsg,
		KickMessage:          kickMsg,
		DeleteMessagesOnKick: kickDeleteMessages,
	}, nil
}
