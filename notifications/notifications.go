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
	JoinServerEnabled bool
	JoinServerChannel string
	JoinServerMsg     string

	JoinDMEnabled bool
	JoinDMMsg     string

	LeaveEnabled bool
	LeaveChannel string
	LeaveMsg     string

	PinEnabled bool
	PinChannel string

	TopicEnabled bool
	TopicChannel string
}

var DefaultConfig = &Config{}

func GetConfig(client *redis.Client, server string) *Config {
	var config *Config
	if err := common.GetRedisJson(client, 0, "notifications/general:"+server, &config); err != nil {
		log.Println("Failed retrieving config", err)
		return DefaultConfig
	}
	return config
}
