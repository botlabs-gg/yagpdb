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

	web.CPMux.HandleC(pat.Get("/commands/settings"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandleCommands, "cp_commands")))
	web.CPMux.HandleC(pat.Get("/commands/settings/"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandleCommands, "cp_commands")))
	web.CPMux.HandleC(pat.Post("/commands/settings/general"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandlePostGeneral, "cp_commands")))
	web.CPMux.HandleC(pat.Post("/commands/settings/channels"), web.RequireGuildChannelsMiddleware(web.RenderHandler(HandlePostChannels, "cp_commands")))
}

// Servers the command page with current config
func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	channels := ctx.Value(web.ContextKeyGuildChannels).([]*discordgo.Channel)
	templateData["CommandConfig"] = GetConfig(client, activeGuild.ID, channels)
	return templateData
}

// Handles more general command settings (prefix)
func HandlePostGeneral(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	channels := ctx.Value(web.ContextKeyGuildChannels).([]*discordgo.Channel)

	err := client.Cmd("SET", "command_prefix:"+activeGuild.ID, strings.TrimSpace(r.FormValue("prefix"))).Err
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed saving config", err))
	} else {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved config! :o"))
	}

	config := GetConfig(client, activeGuild.ID, channels)
	templateData["CommandConfig"] = config

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated commands general settings", user.Username, user.ID))

	return templateData
}

// Handles the updating of global and per channel command settings
func HandlePostChannels(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	channels := ctx.Value(web.ContextKeyGuildChannels).([]*discordgo.Channel)

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
		}

		override.OverrideEnabled = r.FormValue(channel.ID+"_override_enabled") == "on"
	}

	// Update the global settings
	for _, cmd := range config.Global {
		// Check for custom switch
		if cmd.Info.Key == "" {
			cmd.CommandEnabled = r.FormValue("global_enabled_"+cmd.Cmd) == "on"
		}
		cmd.AutoDelete = r.FormValue("global_autodelete_"+cmd.Cmd) == "on"
	}

	err := common.SetRedisJson(client, "commands_settings:"+activeGuild.ID, config)
	if web.CheckErr(templateData, err) {
		return templateData
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated commands channel/global settings", user.Username, user.ID))

	templateData["CommandConfig"] = GetConfig(client, activeGuild.ID, channels)

	return templateData
}
