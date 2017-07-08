package customcommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"html/template"
	"net/http"
	"unicode/utf8"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.Parse(FSMustString(false, "/assets/customcommands.html")))

	getHandler := web.ControllerHandler(HandleCommands, "cp_custom_commands")

	web.CPMux.Handle(pat.Get("/customcommands"), getHandler)
	web.CPMux.Handle(pat.Get("/customcommands/"), getHandler)

	newHandler := web.ControllerPostHandler(HandleNewCommand, getHandler, CustomCommand{}, "Created a new custom command")
	web.CPMux.Handle(pat.Post("/customcommands"), newHandler)
	web.CPMux.Handle(pat.Post("/customcommands/"), newHandler)
	web.CPMux.Handle(pat.Post("/customcommands/:cmd/update"), web.ControllerPostHandler(HandleUpdateCommand, getHandler, CustomCommand{}, "Updated a custom command"))
	web.CPMux.Handle(pat.Post("/customcommands/:cmd/delete"), web.ControllerHandler(HandleDeleteCommand, "cp_custom_commands"))
}

func HandleCommands(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, activeGuild, templateData := web.GetBaseCPContextData(r.Context())

	_, ok := templateData["CustomCommands"]
	if !ok {
		commands, _, err := GetCommands(client, activeGuild.ID)
		if err != nil {
			return templateData, err
		}
		templateData["CustomCommands"] = commands
	}

	return templateData, nil
}

func HandleNewCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/customcommands/"

	newCmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	if len(currentCommands) >= MaxCommands {
		return templateData, web.NewPublicError("Max 50 custom commands allowed, if you need more ask on the support server")
	}

	templateData["CustomCommands"] = currentCommands

	newCmd.TriggerType = TriggerTypeFromForm(newCmd.TriggerTypeForm)
	newCmd.ID = highest + 1

	err = newCmd.Save(client, activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	templateData["CustomCommands"] = append(currentCommands, newCmd)
	return templateData, nil
}

func HandleUpdateCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/customcommands/"

	cmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	// Validate that they haven't messed with the id
	exists, _ := client.Cmd("HEXISTS", KeyCommands(activeGuild.ID), cmd.ID).Bool()
	if !exists {
		return templateData, web.NewPublicError("That command dosen't exist?")
	}

	cmd.TriggerType = TriggerTypeFromForm(cmd.TriggerTypeForm)

	err := cmd.Save(client, activeGuild.ID)

	return templateData, err
}

func HandleDeleteCommand(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/customcommands/"

	cmdIndex := pat.Param(r, "cmd")

	err := client.Cmd("HDEL", KeyCommands(activeGuild.ID), cmdIndex).Err
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
