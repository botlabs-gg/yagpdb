package commands

import (
	"database/sql"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/web"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/volatiletech/sqlboiler/types"
	"goji.io"
	"goji.io/pat"
)

type ChannelOverrideForm struct {
	Channels                []int64 `valid:"channel,true`
	ChannelCategories       []int64 `valid:"channel,true`
	Global                  bool
	CommandsEnabled         bool
	AutodeleteResponse      bool
	AutodeleteTrigger       bool
	AutodeleteResponseDelay int
	AutodeleteTriggerDelay  int
	RequireRoles            []int64 `valid:"role,true"`
	IgnoreRoles             []int64 `valid:"role,true"`
}

type CommandOverrideForm struct {
	Commands                []string
	CommandsEnabled         bool
	AutodeleteResponse      bool
	AutodeleteTrigger       bool
	AutodeleteResponseDelay int
	AutodeleteTriggerDelay  int
	RequireRoles            []int64 `valid:"role,true"`
	IgnoreRoles             []int64 `valid:"role,true"`
}

func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../commands/assets/commands.html", "templates/plugins/commands.html")
	web.AddSidebarItem(web.SidebarCategoryCore, &web.SidebarItem{
		Name: "Command settings",
		URL:  "commands/settings",
		Icon: "fas fa-terminal",
	})

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/commands/settings"), subMux)
	web.CPMux.Handle(pat.New("/commands/settings/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)

	getHandler := web.ControllerHandler(HandleCommands, "cp_commands")
	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)
	subMux.Handle(pat.Post("/general"), web.ControllerPostHandler(HandlePostCommands, getHandler, nil, "Updated command prefix"))

	// Channel override handlers
	subMux.Handle(pat.Post("/channel_overrides/new"),
		web.ControllerPostHandler(HandleCreateChannelsOverride, getHandler, ChannelOverrideForm{}, "Created a new command channels override"))

	subMux.Handle(pat.Post("/channel_overrides/:channelOverride/update"),
		web.ControllerPostHandler(ChannelOverrideMiddleware(HandleUpdateChannelsOverride), getHandler, ChannelOverrideForm{}, "Updated a commands channel override"))

	subMux.Handle(pat.Post("/channel_overrides/:channelOverride/delete"),
		web.ControllerPostHandler(ChannelOverrideMiddleware(HandleDeleteChannelsOverride), getHandler, nil, "Deleted a commands channel override"))

	// Command override handlers
	subMux.Handle(pat.Post("/channel_overrides/:channelOverride/command_overrides/new"),
		web.ControllerPostHandler(ChannelOverrideMiddleware(HandleCreateCommandOverride), getHandler, CommandOverrideForm{}, "Created a commands command override"))

	subMux.Handle(pat.Post("/channel_overrides/:channelOverride/command_overrides/:commandsOverride/update"),
		web.ControllerPostHandler(ChannelOverrideMiddleware(HandleUpdateCommandOVerride), getHandler, CommandOverrideForm{}, "Updated a commands command override"))

	subMux.Handle(pat.Post("/channel_overrides/:channelOverride/command_overrides/:commandsOverride/delete"),
		web.ControllerPostHandler(ChannelOverrideMiddleware(HandleDeleteCommandOverride), getHandler, nil, "Deleted a commands command override"))

}

// Servers the command page with current config
func HandleCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	type SortedCommands struct {
		Category string
		Commands []string
	}

	// Compile all the commands into a sorted list by category
	commands := make([]*SortedCommands, 0, len(CommandSystem.Root.Commands))
	addCommand := func(cmd *YAGCommand, name string) {
		for _, v := range commands {
			if v.Category == cmd.CmdCategory.Name {
				v.Commands = append(v.Commands, name)
				return
			}
		}

		commands = append(commands, &SortedCommands{
			Category: cmd.CmdCategory.Name,
			Commands: []string{name},
		})
	}

	for _, cmd := range CommandSystem.Root.Commands {
		switch t := cmd.Command.(type) {
		case *YAGCommand:
			if t.HideFromCommandsPage {
				continue
			}
			addCommand(t, cmd.Trigger.Names[0])
		case *dcmd.Container:
			for _, containerCmd := range t.Commands {
				cast := containerCmd.Command.(*YAGCommand)

				if cast.HideFromCommandsPage {
					continue
				}

				addCommand(cast, t.Names[0]+" "+containerCmd.Trigger.Names[0])
			}
		}
	}

	templateData["SortedCommands"] = commands

	channelOverrides, err := models.CommandsChannelsOverrides(qm.Where("guild_id=?", activeGuild.ID), qm.Load("CommandsCommandOverrides")).AllG(r.Context())
	if err != nil {
		return templateData, err
	}

	var global *models.CommandsChannelsOverride
	for i, v := range channelOverrides {
		if v.Global {
			global = v
			channelOverrides = append(channelOverrides[:i], channelOverrides[i+1:]...)
			break
		}
	}

	if global == nil {
		global = &models.CommandsChannelsOverride{
			Global:          true,
			CommandsEnabled: true,
		}
	}

	templateData["GlobalCommandSettings"] = global
	templateData["ChannelOverrides"] = channelOverrides

	prefix, _ := GetCommandPrefix(activeGuild.ID)

	templateData["CommandPrefix"] = prefix

	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/commands/settings"

	return templateData, nil
}

// Handles the updating of global and per channel command settings
func HandlePostCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	newPrefix := r.FormValue("Prefix")
	if len(newPrefix) < 1 || len(newPrefix) > 100 {
		return templateData, web.NewPublicError("Prefix is smaller than 1 or larger than 100 characters")
	}

	common.RedisPool.Do(radix.Cmd(nil, "SET", "command_prefix:"+discordgo.StrID(activeGuild.ID), newPrefix))
	featureflags.MarkGuildDirty(activeGuild.ID)

	return templateData, nil
}

// Channel override handlers
func ChannelOverrideMiddleware(inner func(w http.ResponseWriter, r *http.Request, override *models.CommandsChannelsOverride) (web.TemplateData, error)) web.ControllerHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
		activeGuild := r.Context().Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		var override *models.CommandsChannelsOverride
		var err error

		id := pat.Param(r, "channelOverride")
		if id == "global" {
			override, err = models.CommandsChannelsOverrides(qm.Where("guild_id = ? AND global=true", activeGuild.ID)).OneG(r.Context())
			if err == sql.ErrNoRows {
				override = &models.CommandsChannelsOverride{
					Global:          true,
					GuildID:         activeGuild.ID,
					CommandsEnabled: true,
					Channels:        []int64{},
					RequireRoles:    []int64{},
					IgnoreRoles:     []int64{},
				}

				// Insert it
				err = override.InsertG(r.Context(), boil.Infer())
				if err != nil {
					logger.WithError(err).Error("Failed inserting global commands row")
					// Was inserted somewhere else in the meantime
					override, err = models.CommandsChannelsOverrides(qm.Where("guild_id = ? AND global=true", activeGuild.ID)).OneG(r.Context())
				}
			}
		} else {
			idParsed, _ := strconv.ParseInt(id, 10, 64)
			override, err = models.CommandsChannelsOverrides(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, idParsed)).OneG(r.Context())
		}

		if err != nil {
			return nil, web.NewPublicError("Channels override not found, someone else deledted it in the meantime perhaps? Check control panel logs")
		}

		tmpl, err := inner(w, r, override)
		featureflags.MarkGuildDirty(activeGuild.ID)
		return tmpl, err
	}
}

func HandleCreateChannelsOverride(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	formData := r.Context().Value(common.ContextKeyParsedForm).(*ChannelOverrideForm)

	count, err := models.CommandsChannelsOverrides(qm.Where("guild_id = ?", activeGuild.ID), qm.Where("channels && ?", types.Int64Array(formData.Channels))).CountG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "count")
	}

	if count > 0 {
		return templateData.AddAlerts(web.ErrorAlert("One of the selected channels is already used in another override")), nil
	}

	count, err = models.CommandsChannelsOverrides(qm.Where("guild_id = ?", activeGuild.ID)).CountG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "count2")
	}
	if count > 100 {
		return templateData.AddAlerts(web.ErrorAlert("Max 100 channel overrides allowed")), nil
	}

	model := &models.CommandsChannelsOverride{
		GuildID:                 activeGuild.ID,
		Channels:                formData.Channels,
		ChannelCategories:       formData.ChannelCategories,
		CommandsEnabled:         formData.CommandsEnabled,
		AutodeleteResponse:      formData.AutodeleteResponse,
		AutodeleteTrigger:       formData.AutodeleteTrigger,
		AutodeleteResponseDelay: formData.AutodeleteResponseDelay,
		AutodeleteTriggerDelay:  formData.AutodeleteTriggerDelay,
		RequireRoles:            formData.RequireRoles,
		IgnoreRoles:             formData.IgnoreRoles,
	}

	err = model.InsertG(r.Context(), boil.Infer())
	featureflags.MarkGuildDirty(activeGuild.ID)
	return templateData, errors.WithMessage(err, "InsertG")
}

func HandleUpdateChannelsOverride(w http.ResponseWriter, r *http.Request, currentOverride *models.CommandsChannelsOverride) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	formData := r.Context().Value(common.ContextKeyParsedForm).(*ChannelOverrideForm)

	count, err := models.CommandsChannelsOverrides(
		qm.Where("guild_id = ?", activeGuild.ID), qm.Where("channels && ?", types.Int64Array(formData.Channels)), qm.Where("id != ?", currentOverride.ID)).CountG(r.Context())

	if err != nil {
		return templateData, errors.WithMessage(err, "count")
	}

	if count > 0 {
		return templateData.AddAlerts(web.ErrorAlert("One of the selected channels is already used in another override")), nil
	}

	currentOverride.Channels = formData.Channels
	currentOverride.ChannelCategories = formData.ChannelCategories
	currentOverride.CommandsEnabled = formData.CommandsEnabled
	currentOverride.AutodeleteResponse = formData.AutodeleteResponse
	currentOverride.AutodeleteTrigger = formData.AutodeleteTrigger
	currentOverride.AutodeleteResponseDelay = formData.AutodeleteResponseDelay
	currentOverride.AutodeleteTriggerDelay = formData.AutodeleteTriggerDelay
	currentOverride.RequireRoles = formData.RequireRoles
	currentOverride.IgnoreRoles = formData.IgnoreRoles

	_, err = currentOverride.UpdateG(r.Context(), boil.Infer())
	return templateData, errors.WithMessage(err, "UpdateG")
}

func HandleDeleteChannelsOverride(w http.ResponseWriter, r *http.Request, currentOverride *models.CommandsChannelsOverride) (web.TemplateData, error) {
	_, templateData := web.GetBaseCPContextData(r.Context())

	_, err := currentOverride.DeleteG(r.Context())
	return templateData, errors.WithMessage(err, "DeleteG")
}

// Command handlers
func HandleCreateCommandOverride(w http.ResponseWriter, r *http.Request, channelOverride *models.CommandsChannelsOverride) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	formData := r.Context().Value(common.ContextKeyParsedForm).(*CommandOverrideForm)

	count, err := models.CommandsCommandOverrides(qm.Where("commands_channels_overrides_id = ?", channelOverride.ID), qm.Where("commands && ?", types.StringArray(formData.Commands))).CountG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "count")
	}

	if count > 0 {
		return templateData, web.NewPublicError("One of the selected commands is already used in another command override for this channel override")
	}

	count, err = models.CommandsCommandOverrides(qm.Where("commands_channels_overrides_id = ?", channelOverride.ID)).CountG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "count2")
	}

	if count > 250 {
		return templateData, web.NewPublicError("Max 250 command overrides")
	}

	if len(formData.Commands) < 1 {
		return templateData, web.NewPublicError("No commands specified")
	}

	model := &models.CommandsCommandOverride{
		GuildID:                     activeGuild.ID,
		CommandsChannelsOverridesID: channelOverride.ID,

		Commands:                formData.Commands,
		CommandsEnabled:         formData.CommandsEnabled,
		AutodeleteResponse:      formData.AutodeleteResponse,
		AutodeleteTrigger:       formData.AutodeleteTrigger,
		AutodeleteResponseDelay: formData.AutodeleteResponseDelay,
		AutodeleteTriggerDelay:  formData.AutodeleteTriggerDelay,
		RequireRoles:            formData.RequireRoles,
		IgnoreRoles:             formData.IgnoreRoles,
	}

	err = model.InsertG(r.Context(), boil.Infer())

	return templateData, errors.WithMessage(err, "InsertG")
}
func HandleUpdateCommandOVerride(w http.ResponseWriter, r *http.Request, channelOverride *models.CommandsChannelsOverride) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	id := pat.Param(r, "commandsOverride")
	idParsed, _ := strconv.ParseInt(id, 10, 64)

	override, err := models.CommandsCommandOverrides(qm.Where("id = ?", idParsed), qm.Where("guild_id = ?", activeGuild.ID)).OneG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "query override")
	}

	formData := r.Context().Value(common.ContextKeyParsedForm).(*CommandOverrideForm)
	count, err := models.CommandsCommandOverrides(qm.Where("commands_channels_overrides_id = ?", channelOverride.ID), qm.Where("commands && ?", types.StringArray(formData.Commands)), qm.Where("id != ?", override.ID)).CountG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "count")
	}

	if count > 0 {
		return templateData, web.NewPublicError("One of the selected commands is already used in another command override for this channel override")
	}

	override.Commands = formData.Commands
	override.CommandsEnabled = formData.CommandsEnabled
	override.AutodeleteResponse = formData.AutodeleteResponse
	override.AutodeleteTrigger = formData.AutodeleteTrigger
	override.AutodeleteResponseDelay = formData.AutodeleteResponseDelay
	override.AutodeleteTriggerDelay = formData.AutodeleteTriggerDelay
	override.RequireRoles = formData.RequireRoles
	override.IgnoreRoles = formData.IgnoreRoles

	_, err = override.UpdateG(r.Context(), boil.Infer())

	return templateData, errors.WithMessage(err, "UpdateG")
}

func HandleDeleteCommandOverride(w http.ResponseWriter, r *http.Request, channelOverride *models.CommandsChannelsOverride) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	id := pat.Param(r, "commandsOverride")
	idParsed, _ := strconv.ParseInt(id, 10, 64)

	override, err := models.CommandsCommandOverrides(qm.Where("id = ?", idParsed), qm.Where("guild_id = ?", activeGuild.ID)).OneG(r.Context())
	if err != nil {
		return templateData, errors.WithMessage(err, "query override")
	}

	_, err = override.DeleteG(r.Context())

	return templateData, errors.WithMessage(err, "DeleteG")
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Commands"
	templateData["SettingsPath"] = "/commands/settings"
	templateData["WidgetEnabled"] = true

	prefix, err := GetCommandPrefix(ag.ID)
	if err != nil {
		return templateData, err
	}

	count, err := models.CommandsChannelsOverrides(qm.Where("guild_id=?", ag.ID), qm.Where("global=false")).CountG(r.Context())
	if err != nil {
		return templateData, err
	}

	const format = `<ul>
	<li>Command prefix: <code>%s</code></li>
	<li>Active channel overrides: <code>%d</code></li>
</ul>`

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, html.EscapeString(prefix), count))

	return templateData, nil
}
