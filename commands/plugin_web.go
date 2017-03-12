package commands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"strings"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/commands.html"))

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/commands/settings"), subMux)
	web.CPMux.Handle(pat.New("/commands/settings/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)
	subMux.Use(web.RequireFullGuildMW)

	subMux.Handle(pat.Get(""), web.RenderHandler(HandleCommands, "cp_commands"))
	subMux.Handle(pat.Get("/"), web.RenderHandler(HandleCommands, "cp_commands"))
	subMux.Handle(pat.Post("/general"), web.RenderHandler(HandlePostGeneral, "cp_commands"))
	subMux.Handle(pat.Post("/channels"), web.RenderHandler(HandlePostChannels, "cp_commands"))
}

// Servers the command page with current config
func HandleCommands(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	channels := ctx.Value(common.ContextKeyGuildChannels).([]*discordgo.Channel)
	templateData["CommandConfig"] = GetConfig(client, activeGuild.ID, channels)
	return templateData
}

// Handles more general command settings (prefix)
func HandlePostGeneral(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/commands/settings/"
	channels := ctx.Value(common.ContextKeyGuildChannels).([]*discordgo.Channel)

	err := client.Cmd("SET", "command_prefix:"+activeGuild.ID, strings.TrimSpace(r.FormValue("prefix"))).Err
	web.CheckErr(templateData, err, "Failed saving", web.CtxLogger(r.Context()).Error)

	config := GetConfig(client, activeGuild.ID, channels)
	templateData["CommandConfig"] = config

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Updated general command settings")

	return templateData
}

// Handles the updating of global and per channel command settings
func HandlePostChannels(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/commands/settings"

	channels := ctx.Value(common.ContextKeyGuildChannels).([]*discordgo.Channel)

	config := GetConfig(client, activeGuild.ID, channels)

	// Update all the overrides
	for _, channel := range channels {
		// Find the override
		var override *ChannelOverride
		for _, r := range config.ChannelOverrides {
			if r.Channel == channel.ID {
				override = r
				break
			}
		}

		// A new channel was created in the meantime maybe?
		if override == nil {
			continue
		}

		// Update all the command settings for the override
		for _, overrideCmd := range override.Settings {
			overrideCmd.CommandEnabled = r.FormValue(channel.ID+"_enabled_"+overrideCmd.Cmd) == "on"
			overrideCmd.AutoDelete = r.FormValue(channel.ID+"_autodelete_"+overrideCmd.Cmd) == "on"
			overrideCmd.RequiredRole = r.FormValue(channel.ID + "_required_role_" + overrideCmd.Cmd)
		}

		override.OverrideEnabled = r.FormValue(channel.ID+"_override_enabled") == "on"
	}

	// Update the global settings
	for _, cmd := range config.Global {
		// Check for custom switch
		if cmd.Info.Key == "" {
			cmd.CommandEnabled = r.FormValue("global_enabled_"+cmd.Cmd) == "on"
		}
		cmd.RequiredRole = r.FormValue("global_required_role_" + cmd.Cmd)
		cmd.AutoDelete = r.FormValue("global_autodelete_"+cmd.Cmd) == "on"
	}

	err := common.SetRedisJson(client, "commands_settings:"+activeGuild.ID, config)
	if web.CheckErr(templateData, err, "Failed saving item :'(", web.CtxLogger(ctx).Error) {
		return templateData
	}

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Updated advanced command settings")

	templateData["CommandConfig"] = GetConfig(client, activeGuild.ID, channels)

	return templateData
}
