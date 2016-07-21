package commands

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
}

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/commands.html"))

	cpMux.HandleFuncC(pat.Get("/cp/:server/commands"), HandleCommands)
	cpMux.HandleFuncC(pat.Get("/cp/:server/commands/"), HandleCommands)
	cpMux.HandleFuncC(pat.Post("/cp/:server/commands"), HandlePostCommands)
	cpMux.HandleFuncC(pat.Post("/cp/:server/commands/"), HandlePostCommands)
}

func (p *Plugin) Name() string {
	return "Commands"
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "commands"

	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	templateData["current_config"] = GetConfig(client, activeGuild.ID)

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_commands", templateData))
}

func HandlePostCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	newConfig := &CommandsConfig{
		Prefix: r.FormValue("prefix"),
	}
	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "commands"

	err := common.SetRedisJson(client, 0, "commands:"+activeGuild.ID, newConfig)

	if err != nil {
		newConfig = GetConfig(client, activeGuild.ID)
		templateData.AddAlerts(web.ErrorAlert("Failed saving config", err))
	} else {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved config! :o"))
	}

	templateData["current_config"] = newConfig

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_commands", templateData))

}

type CommandsConfig struct {
	Prefix string `json:"prefix"`
}

func GetConfig(client *redis.Client, guild string) *CommandsConfig {
	var config *CommandsConfig
	err := common.GetRedisJson(client, 0, "commands:"+guild, &config)
	if err != nil {
		return &CommandsConfig{}
	}
	return config
}
