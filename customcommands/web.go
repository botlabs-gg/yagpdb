package customcommands

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"unicode/utf8"
)

func (p *Plugin) InitWeb() {
	if os.Getenv("YAGPDB_CC_DISABLE_REDIS_PQ_MIGRATION") == "" {
		go migrateFromRedis()
	}

	tmplPathSettings := "templates/plugins/customcommands.html"
	if common.Testing {
		tmplPathSettings = "../../customcommands/assets/customcommands.html"
	}

	web.Templates = template.Must(web.Templates.ParseFiles(tmplPathSettings))

	getHandler := web.ControllerHandler(HandleCommands, "cp_custom_commands")

	subMux := goji.SubMux()
	web.CPMux.Handle(pat.New("/customcommands"), subMux)
	web.CPMux.Handle(pat.New("/customcommands/*"), subMux)

	subMux.Use(web.RequireGuildChannelsMiddleware)
	subMux.Use(web.RequireFullGuildMW)

	subMux.Handle(pat.Get(""), getHandler)
	subMux.Handle(pat.Get("/"), getHandler)

	newHandler := web.ControllerPostHandler(HandleNewCommand, getHandler, CustomCommand{}, "Created a new custom command")
	subMux.Handle(pat.Post(""), newHandler)
	subMux.Handle(pat.Post("/"), newHandler)
	subMux.Handle(pat.Post("/:cmd/update"), web.ControllerPostHandler(HandleUpdateCommand, getHandler, CustomCommand{}, "Updated a custom command"))
	subMux.Handle(pat.Post("/:cmd/delete"), web.ControllerHandler(HandleDeleteCommand, "cp_custom_commands"))
}

func HandleCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	_, ok := templateData["CustomCommands"]
	if !ok {
		commands, err := models.CustomCommands(qm.Where("guild_id = ?", activeGuild.ID)).AllG(r.Context())
		if err != nil {
			return templateData, err
		}
		templateData["CustomCommands"] = commands
	}

	return templateData, nil
}

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/customcommands/"

	newCmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	currentCommands, err := models.CustomCommands(qm.Where("guild_id = ?", activeGuild.ID)).AllG(ctx)
	if err != nil {
		return templateData, err
	}

	templateData["CustomCommands"] = currentCommands

	if len(currentCommands) >= MaxCommandsForContext(ctx) {
		return templateData, web.NewPublicError(fmt.Sprintf("Max %d custom commands allowed (or %d for premium servers)", MaxCommands, MaxCommandsPremium))
	}

	localID, err := common.GenLocalIncrID(activeGuild.ID, "custom_command")
	if err != nil {
		return templateData, errors.Wrap(err, "error generating local id")
	}

	dbModel := newCmd.ToDBModel()
	dbModel.GuildID = activeGuild.ID
	dbModel.LocalID = localID
	dbModel.TriggerType = int(TriggerTypeFromForm(newCmd.TriggerTypeForm))

	err = dbModel.InsertG(ctx, boil.Infer())
	if err != nil {
		return templateData, err
	}

	templateData["CustomCommands"] = append(currentCommands, dbModel)
	return templateData, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/customcommands/"

	cmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	// savedCmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", activeGuild.ID, cmd.ID)).OneG(ctx)
	// if err != nil {
	// 	return templateData, err
	// }

	dbModel := cmd.ToDBModel()
	dbModel.GuildID = activeGuild.ID
	dbModel.LocalID = cmd.ID
	dbModel.TriggerType = int(TriggerTypeFromForm(cmd.TriggerTypeForm))

	_, err := dbModel.UpdateG(ctx, boil.Infer())

	return templateData, err
}

func HandleDeleteCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/customcommands/"

	cmdID, err := strconv.ParseInt(pat.Param(r, "cmd"), 10, 64)
	if err != nil {
		return templateData, err
	}

	_, err = models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", activeGuild.ID, cmdID)).DeleteAll(ctx, common.PQ)
	if err != nil {
		return templateData, err
	}

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Deleted custom command #"+strconv.FormatInt(cmdID, 10))

	return HandleCommands(w, r)
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
