package customcommands

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
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

	web.CPMux.HandleC(pat.Get("/customcommands"), web.RenderHandler(HandleCommands, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Get("/customcommands/"), web.RenderHandler(HandleCommands, "cp_custom_commands"))

	// If only html allowed patch and delete.. if only
	web.CPMux.HandleC(pat.Post("/customcommands"), web.FormParserMW(web.RenderHandler(HandleNewCommand, "cp_custom_commands"), CustomCommand{}))
	web.CPMux.HandleC(pat.Post("/customcommands/"), web.FormParserMW(web.RenderHandler(HandleNewCommand, "cp_custom_commands"), CustomCommand{}))
	web.CPMux.HandleC(pat.Post("/customcommands/:cmd/update"), web.FormParserMW(web.RenderHandler(HandleUpdateCommand, "cp_custom_commands"), CustomCommand{}))
	web.CPMux.HandleC(pat.Post("/customcommands/:cmd/delete"), web.RenderHandler(HandleDeleteCommand, "cp_custom_commands"))
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	commands, _, err := GetCommands(client, activeGuild.ID)
	if !web.CheckErr(templateData, err, "Failed retrieving commands", logrus.Error) {
		templateData["CustomCommands"] = commands
	}

	return templateData
}

func HandleNewCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	newCmd := ctx.Value(web.ContextKeyParsedForm).(*CustomCommand)
	ok := ctx.Value(web.ContextKeyFormOk).(bool)

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if web.CheckErr(templateData, err, "Failed retrieving commands", logrus.Error) {
		return templateData
	}

	if len(currentCommands) > 49 {
		return templateData.AddAlerts(web.ErrorAlert("Max 50 custom commands allowed"))
	}

	templateData["CustomCommands"] = currentCommands

	if !ok {
		return templateData
	}

	newCmd.TriggerType = TriggerTypeFromForm(newCmd.TriggerTypeForm)

	newCmd.ID = highest + 1

	serialized, err := json.Marshal(newCmd)
	if err != nil {
		logrus.WithError(err).Error("Failed marshaling custom command")
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command"))
	}

	err = client.Cmd("HSET", "custom_commands:"+activeGuild.ID, newCmd.ID, serialized).Err
	if web.CheckErr(templateData, err, "Failed adding command :(", logrus.Error) {
		return templateData
	}

	templateData["CustomCommands"] = append(currentCommands, newCmd)
	templateData.AddAlerts(web.SucessAlert("Sucessfully added command"))

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	go common.AddCPLogEntry(user, activeGuild.ID, "Added new cusom command #", newCmd.ID)

	return templateData
}

func HandleUpdateCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	newCmd := ctx.Value(web.ContextKeyParsedForm).(*CustomCommand)
	ok := ctx.Value(web.ContextKeyFormOk).(bool)
	if !ok {
		return templateData
	}

	newCmd.TriggerType = TriggerTypeFromForm(newCmd.TriggerTypeForm)

	serialized, err := json.Marshal(newCmd)
	if web.CheckErr(templateData, err, "Failed marshaling custom command", logrus.Error) {
		return templateData
	}

	err = client.Cmd("HSET", "custom_commands:"+activeGuild.ID, newCmd.ID, serialized).Err
	if web.CheckErr(templateData, err, "Failed saving command", logrus.Error) {
		return templateData
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Updated command #", newCmd.ID)

	return HandleCommands(ctx, w, r)
}

func HandleDeleteCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	cmdIndex := pat.Param(ctx, "cmd")

	err := client.Cmd("HDEL", "custom_commands:"+activeGuild.ID, cmdIndex).Err
	if web.CheckErr(templateData, err, "Failed deleting command", logrus.Error) {
		return templateData
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Deleted command #"+cmdIndex)

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
