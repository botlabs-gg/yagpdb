package commands

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strings"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	web.RegisterPlugin(plugin)
	bot.RegisterPlugin(plugin)
}

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/commands.html"))

	cpMux.HandleFuncC(pat.Get("/cp/:server/commands/settings"), HandleCommands)
	cpMux.HandleFuncC(pat.Get("/cp/:server/commands/settings/"), HandleCommands)
	cpMux.HandleFuncC(pat.Post("/cp/:server/commands/settings"), HandlePostCommands)
	cpMux.HandleFuncC(pat.Post("/cp/:server/commands/settings/"), HandlePostCommands)
}

func (p *Plugin) Name() string {
	return "Commands"
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "commands"

	templateData["current_config"] = GetConfig(client, activeGuild.ID)

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_commands", templateData))
}

func HandlePostCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "commands"

	newConfig := &CommandsConfig{
		Prefix: strings.TrimSpace(r.FormValue("prefix")),
	}

	err := common.SetRedisJson(client, "commands:"+activeGuild.ID, newConfig)

	if err != nil {
		newConfig = GetConfig(client, activeGuild.ID)
		templateData.AddAlerts(web.ErrorAlert("Failed saving config", err))
	} else {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved config! :o"))
	}

	templateData["current_config"] = newConfig

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated commands settings", user.Username, user.ID))

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_commands", templateData))
}

type CommandsConfig struct {
	Prefix string `json:"prefix"`
}

func GetConfig(client *redis.Client, guild string) *CommandsConfig {
	var config *CommandsConfig
	err := common.GetRedisJson(client, "commands:"+guild, &config)
	if err != nil {
		return &CommandsConfig{}
	}
	return config
}
