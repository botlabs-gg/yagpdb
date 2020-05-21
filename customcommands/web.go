package customcommands

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/volatiletech/null"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/pubsub"
	yagtemplate "github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

// GroupForm is the form bindings used when creating or updating groups
type GroupForm struct {
	ID                int64
	Name              string  `valid:",100"`
	WhitelistChannels []int64 `valid:"channel,true"`
	BlacklistChannels []int64 `valid:"channel,true"`

	WhitelistRoles []int64 `valid:"role,true"`
	BlacklistRoles []int64 `valid:"role,true"`
}

// InitWeb implements web.Plugin
func (p *Plugin) InitWeb() {
	web.LoadHTMLTemplate("../../customcommands/assets/customcommands.html", "templates/plugins/customcommands.html")
	web.LoadHTMLTemplate("../../customcommands/assets/customcommands-editcmd.html", "templates/plugins/customcommands-editcmd.html")
	web.AddSidebarItem(web.SidebarCategoryCore, &web.SidebarItem{
		Name: "Custom commands",
		URL:  "customcommands",
		Icon: "fas fa-closed-captioning",
	})

	getHandler := web.ControllerHandler(handleCommands, "cp_custom_commands")
	getCmdHandler := web.ControllerHandler(handleGetCommand, "cp_custom_commands_edit_cmd")
	getGroupHandler := web.ControllerHandler(handleGetCommandsGroup, "cp_custom_commands")

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/customcommands"), subMux)
	web.CPMux.Handle(pat.New("/customcommands/*"), subMux)

	subMux.Use(func(inner http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			_, templateData := web.GetBaseCPContextData(r.Context())
			strTriggerTypes := map[int]string{}
			for k, v := range triggerStrings {
				strTriggerTypes[int(k)] = v
			}
			templateData["CCTriggerTypes"] = strTriggerTypes

			inner.ServeHTTP(w, r)
		}
		return http.HandlerFunc(h)
	})
	subMux.Use(web.RequireGuildChannelsMiddleware)

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)

	subMux.Handle(pat.Get("/commands/:cmd/"), getCmdHandler)

	subMux.Handle(pat.Get("/groups/:group/"), web.ControllerHandler(handleGetCommandsGroup, "cp_custom_commands"))
	subMux.Handle(pat.Get("/groups/:group"), web.ControllerHandler(handleGetCommandsGroup, "cp_custom_commands"))

	newCommandHandler := web.ControllerPostHandler(handleNewCommand, nil, nil, "Created a new custom command")
	subMux.Handle(pat.Post("/commands/new"), newCommandHandler)
	subMux.Handle(pat.Post("/commands/:cmd/update"), web.ControllerPostHandler(handleUpdateCommand, getCmdHandler, CustomCommand{}, "Updated a custom command"))
	subMux.Handle(pat.Post("/commands/:cmd/delete"), web.ControllerPostHandler(handleDeleteCommand, getHandler, nil, "Deleted a custom command"))

	subMux.Handle(pat.Post("/creategroup"), web.ControllerPostHandler(handleNewGroup, getHandler, GroupForm{}, "Created a new custom command group"))
	subMux.Handle(pat.Post("/groups/:group/update"), web.ControllerPostHandler(handleUpdateGroup, getGroupHandler, GroupForm{}, "Updated a custom command group"))
	subMux.Handle(pat.Post("/groups/:group/delete"), web.ControllerPostHandler(handleDeleteGroup, getHandler, nil, "Deleted a custom command group"))
}

func handleCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	groupID := int64(0)
	if v, ok := templateData["CurrentGroupID"]; ok {
		groupID = v.(int64)
	}

	var langBuiltins strings.Builder
	for k := range yagtemplate.StandardFuncMap {
		langBuiltins.WriteString(" " + k)
	}

	templateData["HLJSBuiltins"] = langBuiltins.String()

	return serveGroupSelected(r, templateData, groupID, activeGuild.ID)
}

func handleGetCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	ccID, err := strconv.ParseInt(pat.Param(r, "cmd"), 10, 64)
	if err != nil {
		return templateData, errors.WithStackIf(err)
	}

	cc, err := models.CustomCommands(
		models.CustomCommandWhere.GuildID.EQ(activeGuild.ID),
		models.CustomCommandWhere.LocalID.EQ(ccID)).OneG(r.Context())
	if err != nil {
		return templateData, errors.WithStackIf(err)
	}

	templateData["CC"] = cc
	templateData["Commands"] = true

	return serveGroupSelected(r, templateData, cc.GroupID.Int64, activeGuild.ID)
}

func handleGetCommandsGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	groupID, _ := strconv.ParseInt(pat.Param(r, "group"), 10, 64)
	return serveGroupSelected(r, templateData, groupID, activeGuild.ID)
}

func serveGroupSelected(r *http.Request, templateData web.TemplateData, groupID int64, guildID int64) (web.TemplateData, error) {
	templateData["GetCCIntervalType"] = tmplGetCCIntervalTriggerType
	templateData["GetCCInterval"] = tmplGetCCInterval

	_, ok := templateData["CustomCommands"]
	if !ok {
		var err error
		var commands []*models.CustomCommand
		if groupID == 0 {
			commands, err = models.CustomCommands(qm.Where("guild_id = ? AND group_id IS NULL", guildID), qm.OrderBy("local_id asc")).AllG(r.Context())
		} else {
			commands, err = models.CustomCommands(qm.Where("guild_id = ? AND group_id = ?", guildID, groupID), qm.OrderBy("local_id asc")).AllG(r.Context())
		}
		if err != nil {
			return templateData, err
		}

		templateData["CustomCommands"] = commands
	}

	commandsGroups, err := models.CustomCommandGroups(qm.Where("guild_id = ?", guildID), qm.OrderBy("id asc")).AllG(r.Context())
	if err != nil {
		return templateData, err
	}

	for _, v := range commandsGroups {
		if v.ID == groupID {
			templateData["CurrentCommandGroup"] = v
			break
		}
	}

	templateData["CommandGroups"] = commandsGroups

	return templateData, nil
}

func handleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	groupID, _ := strconv.ParseInt(r.FormValue("GroupID"), 10, 64)
	if groupID != 0 {
		// make sure we aren't trying to pull any tricks with the group id
		c, err := models.CustomCommandGroups(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, groupID)).CountG(ctx)
		if err != nil {
			return templateData, err
		}

		if c < 1 {
			return templateData.AddAlerts(web.ErrorAlert("Unknown group")), nil
		}

		templateData["CurrentGroupID"] = groupID
	}

	c, err := models.CustomCommands(qm.Where("guild_id = ?", activeGuild.ID)).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if int(c) >= MaxCommandsForContext(ctx) {
		return templateData, web.NewPublicError(fmt.Sprintf("Max %d custom commands allowed (or %d for premium servers)", MaxCommands, MaxCommandsPremium))
	}

	localID, err := common.GenLocalIncrID(activeGuild.ID, "custom_command")
	if err != nil {
		return templateData, errors.WrapIf(err, "error generating local id")
	}

	dbModel := &models.CustomCommand{
		GuildID: activeGuild.ID,
		LocalID: localID,

		Disabled:   true,
		ShowErrors: true,

		TimeTriggerExcludingDays:  []int64{},
		TimeTriggerExcludingHours: []int64{},

		Responses: []string{"Edit this to change the output of the command!"},
	}

	if groupID != 0 {
		dbModel.GroupID = null.Int64From(groupID)
	}

	err = dbModel.InsertG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	featureflags.MarkGuildDirty(activeGuild.ID)

	http.Redirect(w, r, fmt.Sprintf("/manage/%d/customcommands/commands/%d/", activeGuild.ID, localID), http.StatusSeeOther)

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, nil
}

func handleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	cmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	// ensure that the group specified is owned by this guild
	if cmd.GroupID != 0 {
		c, err := models.CustomCommandGroups(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, cmd.GroupID)).CountG(ctx)
		if err != nil {
			return templateData, err
		}

		if c < 1 {
			return templateData.AddAlerts(web.ErrorAlert("Unknown group")), nil
		}
	}

	dbModel := cmd.ToDBModel()

	templateData["CurrentGroupID"] = dbModel.GroupID.Int64

	dbModel.GuildID = activeGuild.ID
	dbModel.LocalID = cmd.ID
	dbModel.TriggerType = int(triggerTypeFromForm(cmd.TriggerTypeForm))

	// check low interval limits
	if dbModel.TriggerType == int(CommandTriggerInterval) && dbModel.TimeTriggerInterval < 10 {
		ok, err := checkIntervalLimits(ctx, activeGuild.ID, dbModel.LocalID, templateData)
		if err != nil || !ok {
			return templateData, err
		}
	}

	_, err := dbModel.UpdateG(ctx, boil.Blacklist("last_run", "next_run", "local_id", "guild_id", "last_error", "last_error_time", "run_count"))
	if err != nil {
		return templateData, nil
	}

	// create, update or remove the next run time and scheduled event
	if dbModel.TriggerType == int(CommandTriggerInterval) {
		// need the last run time
		fullModel, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", activeGuild.ID, dbModel.LocalID)).OneG(ctx)
		if err != nil {
			web.CtxLogger(ctx).WithError(err).Error("failed retrieving full model")
		} else {
			err = UpdateCommandNextRunTime(fullModel, true)
		}
	} else {
		err = DelNextRunEvent(activeGuild.ID, dbModel.LocalID)
	}

	if err != nil {
		web.CtxLogger(ctx).WithError(err).WithField("guild", dbModel.GuildID).Error("failed updating next custom command run time")
	}

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, err
}

func handleDeleteCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	cmdID, err := strconv.ParseInt(pat.Param(r, "cmd"), 10, 64)
	if err != nil {
		return templateData, err
	}

	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", activeGuild.ID, cmdID)).OneG(ctx)
	if err != nil {
		return templateData, err
	}

	groupID := cmd.GroupID.Int64
	if groupID != 0 {
		templateData["CurrentGroupID"] = groupID
	}

	_, err = cmd.DeleteG(ctx)
	if err != nil {
		return templateData, err
	}

	err = DelNextRunEvent(cmd.GuildID, cmd.LocalID)
	featureflags.MarkGuildDirty(activeGuild.ID)
	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, err
}

// allow for max 5 triggers with intervals of less than 10 minutes
func checkIntervalLimits(ctx context.Context, guildID int64, cmdID int64, templateData web.TemplateData) (ok bool, err error) {
	num, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id != ? AND trigger_type = 5 AND time_trigger_interval < 10", guildID, cmdID)).CountG(ctx)
	if err != nil {
		return false, err
	}

	if num < 5 {
		return true, nil
	}

	templateData.AddAlerts(web.ErrorAlert("You can have max 5 triggers on less than 10 minute intervals"))
	return false, nil
}

func handleNewGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	newGroup := ctx.Value(common.ContextKeyParsedForm).(*GroupForm)

	numCurrentGroups, err := models.CustomCommandGroups(qm.Where("guild_id = ?", activeGuild.ID)).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if numCurrentGroups >= MaxGroups {
		return templateData, web.NewPublicError(fmt.Sprintf("Max %d custom command groups", MaxGroups))
	}

	dbModel := &models.CustomCommandGroup{
		GuildID: activeGuild.ID,
		Name:    newGroup.Name,
	}

	err = dbModel.InsertG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	templateData["CurrentGroupID"] = dbModel.ID

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, nil
}

func handleUpdateGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	groupForm := ctx.Value(common.ContextKeyParsedForm).(*GroupForm)

	id, _ := strconv.ParseInt(pat.Param(r, "group"), 10, 64)
	model, err := models.CustomCommandGroups(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, id)).OneG(ctx)
	if err != nil {
		return templateData, err
	}

	model.WhitelistChannels = groupForm.WhitelistChannels
	model.IgnoreChannels = groupForm.BlacklistChannels
	model.WhitelistRoles = groupForm.WhitelistRoles
	model.IgnoreRoles = groupForm.BlacklistRoles
	model.Name = groupForm.Name

	_, err = model.UpdateG(ctx, boil.Infer())

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, err
}

func handleDeleteGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	id, err := strconv.ParseInt(pat.Param(r, "group"), 10, 64)
	if err != nil {
		return templateData, err
	}

	_, err = models.CustomCommandGroups(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, id)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return templateData, err
	}

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, err
}

func triggerTypeFromForm(str string) CommandTriggerType {
	switch str {
	case "none":
		return CommandTriggerNone
	case "prefix":
		return CommandTriggerStartsWith
	case "regex":
		return CommandTriggerRegex
	case "contains":
		return CommandTriggerContains
	case "exact":
		return CommandTriggerExact
	case "command":
		return CommandTriggerCommand
	case "reaction":
		return CommandTriggerReaction
	case "interval_minutes", "interval_hours":
		return CommandTriggerInterval
	default:
		return CommandTriggerCommand

	}
}

func CheckLimits(in ...string) bool {
	for _, v := range in {
		if utf8.RuneCountInString(v) > 2000 {
			return false
		}
	}
	return true
}

// returns 1 for hours, 0 for minutes, -1 otherwise
func tmplGetCCIntervalTriggerType(cc *models.CustomCommand) int {
	if cc.TriggerType != int(CommandTriggerInterval) {
		return -1
	}

	if (cc.TimeTriggerInterval % 60) == 0 {
		return 1
	}

	return 0
}

// returns the proper interval number dispalyed, depending on if it can be rounded to hours or not
func tmplGetCCInterval(cc *models.CustomCommand) int {
	if tmplGetCCIntervalTriggerType(cc) == 1 {
		return cc.TimeTriggerInterval / 60
	}

	return cc.TimeTriggerInterval
}

var _ web.PluginWithServerHomeWidget = (*Plugin)(nil)

func (p *Plugin) LoadServerHomeWidget(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ag, templateData := web.GetBaseCPContextData(r.Context())

	templateData["WidgetTitle"] = "Custom Commands"
	templateData["SettingsPath"] = "/customcommands"

	numCustomCommands, err := models.CustomCommands(qm.Where("guild_id = ?", ag.ID)).CountG(r.Context())

	format := `<p>Number of custom commands: <code>%d</code></p>`

	templateData["WidgetBody"] = template.HTML(fmt.Sprintf(format, numCustomCommands))

	if numCustomCommands > 0 {
		templateData["WidgetEnabled"] = true
	} else {
		templateData["WidgetDisabled"] = true
	}

	return templateData, err
}
