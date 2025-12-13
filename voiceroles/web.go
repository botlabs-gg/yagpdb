package voiceroles

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/streaming"
	"github.com/botlabs-gg/yagpdb/v2/voiceroles/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"goji.io"
	"goji.io/pat"
)

//go:embed assets/voiceroles.html
var PageHTML string

type FormVoiceRole struct {
	ID        int64
	ChannelID int64 `valid:"channel,false"`
	RoleID    int64 `valid:"role,false"`
	Enabled   bool
}

var (
	panelLogKeyNewConfig     = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "voicerole_new_config", FormatString: "Created voice role config for channel %d with role %d"})
	panelLogKeyUpdatedConfig = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "voicerole_updated_config", FormatString: "Updated voice role config %d"})
	panelLogKeyRemovedConfig = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "voicerole_removed_config", FormatString: "Removed voice role config %d"})
)

// getExcludedRoleIDs returns the IDs of roles that should be excluded from voice role operations
// This includes AutoRole, MuteRole, and StreamingRole
func getExcludedRoleIDs(guildID int64) []int64 {
	var excluded []int64

	if autoroleConfig, err := autorole.GetAutoroleConfig(guildID); err == nil && autoroleConfig.Role != 0 {
		excluded = append(excluded, autoroleConfig.Role)
	}

	if modConfig, err := moderation.FetchConfig(guildID); err == nil && modConfig.MuteRole != 0 {
		excluded = append(excluded, modConfig.MuteRole)
	}

	if streamingConfig, err := streaming.GetConfig(guildID); err == nil && streamingConfig.GiveRole != 0 {
		excluded = append(excluded, streamingConfig.GiveRole)
	}

	return excluded
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("voiceroles/assets/voiceroles.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryRoles, &web.SidebarItem{
		Name: "Voice Roles",
		URL:  "voiceroles",
		Icon: "fas fa-microphone",
	})

	muxer := goji.SubMux()

	web.CPMux.Handle(pat.New("/voiceroles"), muxer)
	web.CPMux.Handle(pat.New("/voiceroles/*"), muxer)

	muxer.Use(web.RequireBotMemberMW)
	muxer.Use(premium.PremiumGuildMW)
	muxer.Use(web.RequirePermMW(discordgo.PermissionManageRoles))

	getHandler := web.RenderHandler(handleGetVoiceRole, "cp_voiceroles")

	muxer.Handle(pat.Get(""), getHandler)
	muxer.Handle(pat.Get("/"), getHandler)

	muxer.Handle(pat.Post("/new"), web.ControllerPostHandler(handleNewVoiceRole, getHandler, FormVoiceRole{}))
	muxer.Handle(pat.Post("/:id/update"), web.ControllerPostHandler(handleUpdateVoiceRole, getHandler, FormVoiceRole{}))
	muxer.Handle(pat.Post("/:id/delete"), web.ControllerPostHandler(handleDeleteVoiceRole, getHandler, nil))
}

func handleGetVoiceRole(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	configs, err := GetVoiceRoles(ctx, activeGuild.ID)
	web.CheckErr(tmpl, err, "Failed retrieving voice role configs", logger.Error)

	// Enrich configs with channel and role names
	type EnrichedConfig struct {
		*models.VoiceRole
		ChannelName string
		RoleName    string
	}

	enrichedConfigs := make([]*EnrichedConfig, 0, len(*configs))
	for _, config := range *configs {
		enriched := &EnrichedConfig{
			VoiceRole:   config,
			ChannelName: "Unknown Channel",
			RoleName:    "Unknown Role",
		}

		// Get channel name
		if channel := activeGuild.GetChannel(config.ChannelID); channel != nil {
			enriched.ChannelName = channel.Name
		}

		// Get role name
		if role := activeGuild.GetRole(config.RoleID); role != nil {
			enriched.RoleName = role.Name
		}

		enrichedConfigs = append(enrichedConfigs, enriched)
	}

	tmpl["VoiceRoles"] = enrichedConfigs

	// Pass excluded role IDs to template
	excludedRoleIDs := getExcludedRoleIDs(activeGuild.ID)
	tmpl["ExcludedRoleIDs"] = excludedRoleIDs

	// Calculate limits
	configCount := len(*configs)
	tmpl["ConfigCount"] = configCount

	// Check enabled count for limits
	enabledCount := 0
	for _, c := range *configs {
		if c.Enabled {
			enabledCount++
		}
	}

	// Max limit is always 10 for storage, but free servers can only enable 1
	maxAllowed := MaxConfigsForContext(ctx)
	tmpl["CanAddMore"] = configCount < maxAllowed
	tmpl["MaxConfigs"] = maxAllowed
	tmpl["PremiumLimit"] = MaxVoiceRolesPremium
	tmpl["FreeLimit"] = MaxVoiceRoles

	return tmpl
}

func handleNewVoiceRole(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*FormVoiceRole)

	// Validate channel is a voice or stage channel
	channel := activeGuild.GetChannel(form.ChannelID)
	if channel == nil {
		return tmpl.AddAlerts(web.ErrorAlert("Channel not found")), nil
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice && channel.Type != discordgo.ChannelTypeGuildStageVoice {
		return tmpl.AddAlerts(web.ErrorAlert("Selected channel must be a voice or stage channel")), nil
	}

	// Check total limits
	count, err := GetVoiceRolesCount(ctx, activeGuild.ID)
	if err != nil {
		return tmpl, errors.WithMessage(err, "failed getting config count")
	}

	maxAllowed := MaxConfigsForContext(ctx)
	if count >= int64(maxAllowed) {
		return tmpl.AddAlerts(web.ErrorAlert("Maximum of " + strconv.Itoa(maxAllowed) + " voice role configurations reached")), nil
	}

	// Create the config (Enabled by default)
	config, err := CreateVoiceRoles(ctx, activeGuild.ID, form.ChannelID, form.RoleID)
	if err != nil {
		// Check for unique constraint violation
		if common.ErrPQIsUniqueViolation(err) {
			return tmpl.AddAlerts(web.ErrorAlert("A voice role configuration already exists for this channel")), nil
		}
		return tmpl, errors.WithMessage(err, "failed creating voice role config")
	}

	config.Enabled = true
	_, err = config.UpdateG(ctx, boil.Infer())
	if err != nil {
		return tmpl, errors.WithMessage(err, "failed enabling voice role config")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyNewConfig, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: form.ChannelID}, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: form.RoleID}))

	return tmpl.AddAlerts(web.SucessAlert("Voice role configuration created!")), nil
}

func handleUpdateVoiceRole(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*FormVoiceRole)

	idStr := pat.Param(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Invalid ID")), nil
	}

	// Validate channel is a voice or stage channel
	channel := activeGuild.GetChannel(form.ChannelID)
	if channel == nil {
		return tmpl.AddAlerts(web.ErrorAlert("Channel not found")), nil
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice && channel.Type != discordgo.ChannelTypeGuildStageVoice {
		return tmpl.AddAlerts(web.ErrorAlert("Selected channel must be a voice or stage channel")), nil
	}

	// Check concurrent limits if enabling
	if form.Enabled {
		// Get current config to see if it was already enabled
		currentConfig, err := models.FindVoiceRoleG(ctx, id)
		if err != nil {
			return tmpl, errors.WithMessage(err, "failed finding config")
		}

		if !currentConfig.Enabled {
			// We are enabling it, check limits
			enabledCount, err := models.VoiceRoles(
				models.VoiceRoleWhere.GuildID.EQ(activeGuild.ID),
				models.VoiceRoleWhere.Enabled.EQ(true),
			).CountG(ctx)
			if err != nil {
				return tmpl, errors.WithMessage(err, "failed counting enabled configs")
			}

			limit := MaxConfigsForContext(ctx)
			if int(enabledCount) >= limit {
				return tmpl.AddAlerts(web.ErrorAlert(fmt.Sprintf("Max %d enabled voice roles allowed", limit))), nil
			}
		}
	}

	// Update the config
	config, err := models.FindVoiceRoleG(ctx, id)
	if err != nil {
		return tmpl, errors.WithMessage(err, "failed finding config")
	}

	config.ChannelID = form.ChannelID
	config.RoleID = form.RoleID
	config.Enabled = form.Enabled

	_, err = config.UpdateG(ctx, boil.Infer())

	if err != nil {
		// Check for unique constraint violation
		if common.ErrPQIsUniqueViolation(err) {
			return tmpl.AddAlerts(web.ErrorAlert("A voice role already exists for this channel")), nil
		}
		return tmpl, errors.WithMessage(err, "failed updating voice role config")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyUpdatedConfig, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: id}))

	return tmpl.AddAlerts(web.SucessAlert("Voice role updated!")), nil
}

func handleDeleteVoiceRole(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	_, tmpl := web.GetBaseCPContextData(ctx)

	idStr := pat.Param(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Invalid ID")), nil
	}

	err = DeleteVoiceRoles(ctx, id)
	if err != nil {
		return tmpl, errors.WithMessage(err, "failed deleting voice role config")
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKeyRemovedConfig, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: id}))

	return tmpl.AddAlerts(web.SucessAlert("Voice role deleted!")), nil
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Voice Role"
	templateData["SettingsPath"] = "/voiceroles"

	configs, err := GetVoiceRoles(r.Context(), ag.ID)
	if err != nil {
		return templateData, err
	}

	if len(*configs) > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	format := `<ul>
	<li>Status: %s</li>
	<li>Voice Roles: <code>%d</code></li>
</ul>`

	status := web.EnabledDisabledSpanStatus(len(*configs) > 0)
	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, status, len(*configs)))

	return templateData, nil
}
