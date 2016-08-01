package moderation

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"log"
	"net/http"
)

func (p *Plugin) InitWeb(rootMux, cpMux *goji.Mux) {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/moderation_commands.html"))

	cpMux.HandleFuncC(pat.Get("/cp/:server/commands/moderation"), HandleModeration)
	cpMux.HandleFuncC(pat.Get("/cp/:server/commands/moderation/"), HandleModeration)
	cpMux.HandleFuncC(pat.Post("/cp/:server/commands/moderation"), HandlePostModeration)
}

// The moderation page itself
func HandleModeration(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "moderation"
	config, err := GetConfig(client, activeGuild.ID)

	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving config", err))
		log.Println("Failed retrieving config", err)
	}

	templateData["current_config"] = config

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_moderation_commands", templateData))
}

// Update the settings
func HandlePostModeration(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "moderation"

	r.ParseForm()

	newConfig := &Config{
		BanEnabled:    r.FormValue("ban_enabled") == "on",
		KickEnabled:   r.FormValue("kick_enabled") == "on",
		ReportEnabled: r.FormValue("report_enabled") == "on",
		CleanEnabled:  r.FormValue("clean_enabled") == "on",
	}

	channels, err := common.GetGuildChannels(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed updating channels", err))
		log.Println("Failed updating channels", err)
	} else {
		// Make sure the channel is on the desired guild
		for _, c := range channels {
			if c.ID == r.FormValue("report_channel") {
				newConfig.ReportChannel = c.ID
			}
			if c.ID == r.FormValue("action_channel") {
				newConfig.ActionChannel = c.ID
			}
		}
	}

	err = newConfig.Save(client, activeGuild.ID)

	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed saving config", err))
	} else {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved config! :o"))
	}

	templateData["current_config"] = newConfig

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated moderation settings", user.Username, user.ID))

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_moderation_commands", templateData))
}
