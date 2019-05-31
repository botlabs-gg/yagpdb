package customcommands

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
)

type GroupForm struct {
	ID                int64
	Name              string  `valid:",100"`
	WhitelistChannels []int64 `valid:"channel,true"`
	BlacklistChannels []int64 `valid:"channel,true"`

	WhitelistRoles []int64 `valid:"role,true"`
	BlacklistRoles []int64 `valid:"role,true"`
}

func (p *Plugin) InitWeb() {
	tmplPathSettings := "templates/plugins/customcommands.html"
	if common.Testing {
		tmplPathSettings = "../../customcommands/assets/customcommands.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	getHandler := web.ControllerHandler(HandleCommands, "cp_custom_commands")
	getGroupHandler := web.ControllerHandler(HandleGetCommandsGroup, "cp_custom_commands")

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/customcommands"), subMux)
	web.CPMux.Handle(pat.New("/customcommands/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)

	subMux.Handle(pat.Get("/groups/:group/"), web.ControllerHandler(HandleGetCommandsGroup, "cp_custom_commands"))
	subMux.Handle(pat.Get("/groups/:group"), web.ControllerHandler(HandleGetCommandsGroup, "cp_custom_commands"))

	newCommandHandler := web.ControllerPostHandler(HandleNewCommand, getHandler, CustomCommand{}, "Created a new custom command")
	subMux.Handle(pat.Post("/createcommand"), newCommandHandler)
	subMux.Handle(pat.Post("/commands/:cmd/update"), web.ControllerPostHandler(HandleUpdateCommand, getHandler, CustomCommand{}, "Updated a custom command"))
	subMux.Handle(pat.Post("/commands/:cmd/delete"), web.ControllerPostHandler(HandleDeleteCommand, getHandler, nil, "Deleted a custom command"))

	subMux.Handle(pat.Post("/creategroup"), web.ControllerPostHandler(HandleNewGroup, getHandler, GroupForm{}, "Created a new custom command group"))
	subMux.Handle(pat.Post("/groups/:group/update"), web.ControllerPostHandler(HandleUpdateGroup, getGroupHandler, GroupForm{}, "Updated a custom command group"))
	subMux.Handle(pat.Post("/groups/:group/delete"), web.ControllerPostHandler(HandleDeleteGroup, getHandler, nil, "Deleted a custom command group"))
}

func HandleCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	groupID := int64(0)
	if v, ok := templateData["CurrentGroupID"]; ok {
		groupID = v.(int64)
	}

	return ServeGroupSelected(r, templateData, groupID, activeGuild.ID)
}

func HandleGetCommandsGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())
	groupID, _ := strconv.ParseInt(pat.Param(r, "group"), 10, 64)
	return ServeGroupSelected(r, templateData, groupID, activeGuild.ID)
}

func ServeGroupSelected(r *http.Request, templateData web.TemplateData, groupID int64, guildID int64) (web.TemplateData, error) {
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

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	newCmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	c, err := models.CustomCommands(qm.Where("guild_id = ?", activeGuild.ID)).CountG(ctx)
	if err != nil {
		return templateData, err
	}

	if int(c) >= MaxCommandsForContext(ctx) {
		return templateData, web.NewPublicError(fmt.Sprintf("Max %d custom commands allowed (or %d for premium servers)", MaxCommands, MaxCommandsPremium))
	}

	// ensure that the group specified is owned by this guild
	if newCmd.GroupID != 0 {
		c, err := models.CustomCommandGroups(qm.Where("guild_id = ? AND id = ?", activeGuild.ID, newCmd.GroupID)).CountG(ctx)
		if err != nil {
			return templateData, err
		}

		if c < 1 {
			return templateData.AddAlerts(web.ErrorAlert("Unknown group")), nil
		}

		templateData["CurrentGroupID"] = newCmd.GroupID
	}

	localID, err := common.GenLocalIncrID(activeGuild.ID, "custom_command")
	if err != nil {
		return templateData, errors.Wrap(err, "error generating local id")
	}

	dbModel := newCmd.ToDBModel()
	dbModel.GuildID = activeGuild.ID
	dbModel.LocalID = localID
	dbModel.TriggerType = int(TriggerTypeFromForm(newCmd.TriggerTypeForm))

	// check low interval limits
	if dbModel.TriggerType == int(CommandTriggerInterval) && dbModel.TimeTriggerInterval < 10 {
		ok, err := CheckIntervalLimits(ctx, activeGuild.ID, -1, templateData)
		if err != nil || !ok {
			return templateData, err
		}
	}

	err = dbModel.InsertG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	if dbModel.TriggerType == int(CommandTriggerInterval) {
		// create, update or remove the next run time and scheduled event
		err = UpdateCommandNextRunTime(dbModel, true)
		if err != nil {
			web.CtxLogger(ctx).WithError(err).WithField("guild", dbModel.GuildID).Error("failed updating next custom command run time")
		}
	}

	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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
	dbModel.TriggerType = int(TriggerTypeFromForm(cmd.TriggerTypeForm))

	// check low interval limits
	if dbModel.TriggerType == int(CommandTriggerInterval) && dbModel.TimeTriggerInterval < 10 {
		ok, err := CheckIntervalLimits(ctx, activeGuild.ID, dbModel.LocalID, templateData)
		if err != nil || !ok {
			return templateData, err
		}
	}

	_, err := dbModel.UpdateG(ctx, boil.Blacklist("last_run", "next_run", "local_id", "guild_id"))
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

func HandleDeleteCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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
	common.LogIgnoreError(pubsub.Publish("custom_commands_clear_cache", activeGuild.ID, nil), "failed creating pubsub cache eviction event", web.CtxLogger(ctx).Data)
	return templateData, err
}

// allow for max 5 triggers with intervals of less than 10 minutes
func CheckIntervalLimits(ctx context.Context, guildID int64, cmdID int64, templateData web.TemplateData) (ok bool, err error) {
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

func HandleNewGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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

func HandleUpdateGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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

func HandleDeleteGroup(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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

func TriggerTypeFromForm(str string) CommandTriggerType {
	switch str {
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
