package customcommands

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
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

	web.CPMux.HandleC(pat.Get("/cp/:server/customcommands"), web.RenderHandler(HandleCommands, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Get("/cp/:server/customcommands/"), web.RenderHandler(HandleCommands, "cp_custom_commands"))

	// If only html allowed patch and delete.. if only
	web.CPMux.HandleC(pat.Post("/cp/:server/customcommands"), web.RenderHandler(HandleNewCommand, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Post("/cp/:server/customcommands/:cmd/update"), web.RenderHandler(HandleUpdateCommand, "cp_custom_commands"))
	web.CPMux.HandleC(pat.Post("/cp/:server/customcommands/:cmd/delete"), web.RenderHandler(HandleDeleteCommand, "cp_custom_commands"))
}

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	commands, _, err := GetCommands(client, activeGuild.ID)
	if !web.CheckErr(templateData, err) {
		templateData["commands"] = commands
	}

	return templateData
}

func HandleNewCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	r.ParseForm()

	triggerType := TriggerTypeFromForm(r.FormValue("type"))

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if web.CheckErr(templateData, err) {
		return templateData
	}

	templateData["commands"] = currentCommands

	trigger := r.FormValue("trigger")
	response := r.FormValue("response")
	if !CheckLimits(trigger, response) {
		return templateData.AddAlerts(web.ErrorAlert("TOOOOO Big boi"))
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
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
	}

	redisCommands := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"custom_commands:" + activeGuild.ID, cmd.ID, serialized}},
	}

	_, err = common.SafeRedisCommands(client, redisCommands)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
	}

	templateData["commands"] = append(currentCommands, cmd)
	templateData.AddAlerts(web.SucessAlert("Sucessfully added command"))

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Added a new custom command", user.Username, user.ID))

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
		return templateData.AddAlerts(web.ErrorAlert("TOOOOO Big boi"))
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
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
	}

	redisCommands := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"custom_commands:" + activeGuild.ID, cmdIndex, serialized}},
	}

	_, err = common.SafeRedisCommands(client, redisCommands)
	if err != nil {
		return templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
	}

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands", err))
	} else {
		templateData["commands"] = commands
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated command #%s", user.Username, user.ID, cmdIndex))

	return templateData
}

func HandleDeleteCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)
	templateData["current_page"] = "custom_commands"

	cmdIndex := pat.Param(ctx, "cmd")

	cmds := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
		&common.RedisCmd{Name: "HDEL", Args: []interface{}{"custom_commands:" + activeGuild.ID, cmdIndex}},
	}

	_, err := common.SafeRedisCommands(client, cmds)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed deleting command", err))
	}

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands", err))
	} else {
		templateData["commands"] = commands
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Deleted command #%s", user.Username, user.ID, cmdIndex))

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
