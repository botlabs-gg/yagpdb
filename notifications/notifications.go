package notifications

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"log"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	bot.RegisterPlugin(plugin)
	web.RegisterPlugin(plugin)
}

func (p *Plugin) Name() string {
	return "Notifications"
}

func (p *Plugin) InitBot() {
	bot.Session.AddHandler(HandleGuildCreate)
	bot.Session.AddHandler(HandleGuildMemberAdd)
	bot.Session.AddHandler(HandleGuildMemberRemove)
	bot.Session.AddHandler(HandleChannelUpdate)
}

func (p *Plugin) InitWeb(mainMuxer, cpMuxer *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/notifications_general.html"))
	cpMuxer.HandleFuncC(pat.Get("/cp/:server/notifications/general"), HandleNotificationsGet)
	cpMuxer.HandleFuncC(pat.Get("/cp/:server/notifications/general/"), HandleNotificationsGet)
	cpMuxer.HandleFuncC(pat.Post("/cp/:server/notifications/general"), HandleNotificationsPost)
	cpMuxer.HandleFuncC(pat.Post("/cp/:server/notifications/general/"), HandleNotificationsPost)
}

type Config struct {
	JoinServerEnabled bool   `json:"join_server_enabled`
	JoinServerChannel string `json:"join_server_channel"`
	JoinServerMsg     string `json:"join_server_msg"`

	JoinDMEnabled bool   `json:"join_dm_enabled"`
	JoinDMMsg     string `json:"join_dm_msg"`

	LeaveEnabled bool   `json:"leave_enabled"`
	LeaveChannel string `json:"leave_channel"`
	LeaveMsg     string `json:"leave_msg"`

	PinEnabled bool   `json:"pin_enabled"`
	PinChannel string `json:"pin_channel"`

	TopicEnabled bool   `json:"topic_enabled"`
	TopicChannel string `json:"topic_channel"`
}

var DefaultConfig = &Config{}

func GetConfig(client *redis.Client, server string) *Config {
	var config *Config
	if err := common.GetRedisJson(client, "notifications/general:"+server, &config); err != nil {
		if _, ok := err.(*redis.CmdError); ok {
			log.Println("Failed retrieving config", err)
		}
		return DefaultConfig
	}
	return config
}
