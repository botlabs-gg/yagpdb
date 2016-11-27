package customcommands

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"html/template"
	"net/http"
	"unicode/utf8"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/custom_commands.html"))

	getHandler := web.ControllerHandler(HandleCommands, "cp_custom_commands")

	web.CPMux.HandleC(pat.Get("/customcommands"), getHandler)
	web.CPMux.HandleC(pat.Get("/customcommands/"), getHandler)

	newHandler := web.ControllerPostHandler(HandleNewCommand, getHandler, CustomCommand{}, "Created a new custom command")
	web.CPMux.HandleC(pat.Post("/customcommands"), newHandler)
	web.CPMux.HandleC(pat.Post("/customcommands/"), newHandler)

	// If only html allowed patch and delete.. if only
	//web.CPMux.HandleC(pat.Post("/customcommands"), web.FormParserMW(web.RenderHandler(HandleNewCommand, "cp_custom_commands"), CustomCommand{}))
	//web.CPMux.HandleC(pat.Post("/customcommands/"), web.FormParserMW(web.RenderHandler(HandleNewCommand, "cp_custom_commands"), CustomCommand{}))
	// web.CPMux.HandleC(pat.Post("/customcommands/:cmd/update"), web.FormParserMW(web.RenderHandler(HandleUpdateCommand, "cp_custom_commands"), CustomCommand{}))
	web.CPMux.HandleC(pat.Post("/customcommands/:cmd/update"), web.ControllerPostHandler(HandleUpdateCommand, getHandler, CustomCommand{}, "Updated a custom command"))
	web.CPMux.HandleC(pat.Post("/customcommands/:cmd/delete"), web.ControllerHandler(HandleDeleteCommand, "cp_custom_commands"))
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

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

func HandleNewCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/customcommands/"

	newCmd := ctx.Value(common.ContextKeyParsedForm).(*CustomCommand)

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	if len(currentCommands) > 49 {
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

func HandleUpdateCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
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

func HandleDeleteCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["VisibleURL"] = "/cp/" + activeGuild.ID + "/customcommands/"

	cmdIndex := pat.Param(ctx, "cmd")

	err := client.Cmd("HDEL", KeyCommands(activeGuild.ID), cmdIndex).Err
	if err != nil {
		return templateData, err
	}

	user := ctx.Value(common.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Deleted command #"+cmdIndex)

	return HandleCommands(ctx, w, r)
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
