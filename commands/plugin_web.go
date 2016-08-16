package commands

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"strings"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/commands.html"))

	web.CPMux.HandleC(pat.Get("/cp/:server/commands/settings"), web.RenderHandler(HandleCommands, "cp_commands"))
	web.CPMux.HandleC(pat.Get("/cp/:server/commands/settings/"), web.RenderHandler(HandleCommands, "cp_commands"))
	web.CPMux.HandleC(pat.Post("/cp/:server/commands/settings"), web.RenderHandler(HandlePostCommands, "cp_commands"))
	web.CPMux.HandleC(pat.Post("/cp/:server/commands/settings/"), web.RenderHandler(HandlePostCommands, "cp_commands"))
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_config"] = GetConfig(client, activeGuild.ID)
	return templateData
}

func HandlePostCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

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

	return templateData
}
