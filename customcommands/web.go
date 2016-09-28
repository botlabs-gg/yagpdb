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
	"strconv"
	"unicode/utf8"
)

func (p *Plugin) InitWeb() {
	web.Templates = template.Must(web.Templates.ParseFiles("templates/plugins/custom_commands.html"))

	web.CPMux.HandleC(pat.Get("/customcommands"), web.RenderHandler(HandleCommands, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Get("/customcommands/"), web.RenderHandler(HandleCommands, "cp_custom_commands"))

	// If only html allowed patch and delete.. if only
	web.CPMux.HandleC(pat.Post("/customcommands"), web.RenderHandler(HandleNewCommand, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Post("/customcommands/:cmd/update"), web.RenderHandler(HandleUpdateCommand, "cp_custom_commands"))
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

	r.ParseForm()

	triggerType := TriggerTypeFromForm(r.FormValue("type"))

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if web.CheckErr(templateData, err, "Failed retrieving commands", logrus.Error) {
		return templateData
	}

	templateData["CustomCommands"] = currentCommands

	if len(currentCommands) > 49 {
		return templateData.AddAlerts(web.ErrorAlert("Max 50 custom commands allowed"))
	}

	trigger := r.FormValue("trigger")
	response := r.FormValue("response")
	if !CheckLimits(trigger, response) {
		return templateData.AddAlerts(web.ErrorAlert("Too big response or trigger"))
	}

	cmd := &CustomCommand{
		TriggerType:   triggerType,
		Trigger:       trigger,
		Response:      response,
		ID:            highest + 1,
		CaseSensitive: r.FormValue("case_sensitive") == "on",
	}

	serialized, err := json.Marshal(cmd)
	if err != nil {
		logrus.WithError(err).Error("Failed marshaling custom command")
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command"))
	}

	err = client.Cmd("HSET", "custom_commands:"+activeGuild.ID, cmd.ID, serialized).Err
	if err != nil {
		logrus.WithError(err).WithField("guild", activeGuild.ID).Error("Failed adding custom command")
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command"))
	}

	templateData["CustomCommands"] = append(currentCommands, cmd)
	templateData.AddAlerts(web.SucessAlert("Sucessfully added command"))

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Added new cusom command #", cmd.ID)

	return templateData
}

func HandleUpdateCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	cmdIndex := pat.Param(ctx, "cmd")

	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	triggerType := TriggerTypeFromForm(r.FormValue("type"))
	id, _ := strconv.ParseInt(cmdIndex, 10, 32)

	trigger := r.FormValue("trigger")
	response := r.FormValue("response")
	if !CheckLimits(trigger, response) {
		return templateData.AddAlerts(web.ErrorAlert("Too big response or trigger"))
	}

	cmd := &CustomCommand{
		TriggerType:   triggerType,
		Trigger:       trigger,
		Response:      response,
		CaseSensitive: r.FormValue("case_sensitive") == "on",
		ID:            int(id),
	}

	serialized, err := json.Marshal(cmd)
	if err != nil {
		logrus.WithError(err).Error("Failed marshaling custom command")
		return templateData.AddAlerts(web.ErrorAlert("Failed updating command"))
	}

	err = client.Cmd("HSET", "custom_commands:"+activeGuild.ID, cmdIndex, serialized).Err
	if err != nil {
		logrus.WithError(err).WithField("guild", activeGuild.ID).Error("Failed updating custom command")
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command"))
	}

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		logrus.WithError(err).WithField("guild", activeGuild.ID).Error("Failed retrieving custom commands")
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands"))
	} else {
		templateData["CustomCommands"] = commands
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Updated command #"+cmdIndex)

	return templateData
}

func HandleDeleteCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "custom_commands"

	cmdIndex := pat.Param(ctx, "cmd")

	err := client.Cmd("HDEL", "custom_commands:"+activeGuild.ID, cmdIndex).Err
	if err != nil {
		logrus.WithError(err).WithField("guild", activeGuild.ID).Error("Failed deleting custom command")
		templateData.AddAlerts(web.ErrorAlert("Failed deleting command"))
	}

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands", err))
	} else {
		templateData["CustomCommands"] = commands
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(user, activeGuild.ID, "Deleted command #"+cmdIndex)

	return templateData
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
