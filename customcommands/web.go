package customcommands

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"github.com/mediocregopher/radix"
	"goji.io"
	"goji.io/pat"
	"html/template"
	"net/http"
	"unicode/utf8"
)

func (p *Plugin) InitWeb() {
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
		commands, _, err := GetCommands(activeGuild.ID)
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

	currentCommands, highest, err := GetCommands(activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	if len(currentCommands) >= MaxCommandsForContext(ctx) {
		return templateData, web.NewPublicError(fmt.Sprintf("Max %d custom commands allowed (or %d for premium servers)", MaxCommands, MaxCommandsPremium))
	}

	templateData["CustomCommands"] = currentCommands

	newCmd.TriggerType = TriggerTypeFromForm(newCmd.TriggerTypeForm)
	newCmd.ID = highest + 1

	err = newCmd.Save(activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	templateData["CustomCommands"] = append(currentCommands, newCmd)
	return templateData, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/customcommands/"

	cmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	// Validate that they haven't messed with the id
	var exists bool
	common.RedisPool.Do(radix.FlatCmd(&exists, "HEXISTS", KeyCommands(activeGuild.ID), cmd.ID))
	if !exists {
		return templateData, web.NewPublicError("That command dosen't exist?")
	}

	cmd.TriggerType = TriggerTypeFromForm(cmd.TriggerTypeForm)

	err := cmd.Save(activeGuild.ID)

	return templateData, err
}

func HandleDeleteCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/manage/" + discordgo.StrID(activeGuild.ID) + "/customcommands/"

	cmdIndex := pat.Param(r, "cmd")

	err := common.RedisPool.Do(radix.Cmd(nil, "HDEL", KeyCommands(activeGuild.ID), cmdIndex))
	if err != nil {
		return templateData, err
	}

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Deleted command #"+cmdIndex)

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
