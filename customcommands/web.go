package customcommands

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"goji.io/pat"
	"golang.org/x/net/context"
	"net/http"
	"strconv"
	"unicode/utf8"
)

func HandleCommands(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "custom_commands"

	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands", err))
	} else {
		templateData["commands"] = commands
	}

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
}

func HandleNewCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "custom_commands"
	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	r.ParseForm()

	triggerType := TriggerTypeFromForm(r.FormValue("type"))

	currentCommands, highest, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}
	templateData["commands"] = currentCommands

	trigger := r.FormValue("trigger")
	response := r.FormValue("response")
	if !CheckLimits(trigger, response) {
		templateData.AddAlerts(web.ErrorAlert("TOOOOO Big boi"))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	cmd := &CustomCommand{
		TriggerType: triggerType,
		Trigger:     trigger,
		Response:    response,
		ID:          highest + 1,
	}
	serialized, err := json.Marshal(cmd)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	redisCommands := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"custom_commands:" + activeGuild.ID, cmd.ID, serialized}},
	}

	_, err = common.SafeRedisCommands(client, redisCommands)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	templateData["commands"] = append(currentCommands, cmd)
	templateData.AddAlerts(web.SucessAlert("Sucessfully added command"))

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
}

func HandleUpdateCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cmdIndex := pat.Param(ctx, "cmd")

	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "custom_commands"
	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	triggerType := TriggerTypeFromForm(r.FormValue("type"))
	id, _ := strconv.ParseInt(cmdIndex, 10, 32)

	trigger := r.FormValue("trigger")
	response := r.FormValue("response")
	if !CheckLimits(trigger, response) {
		templateData.AddAlerts(web.ErrorAlert("TOOOOO Big boi"))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	cmd := &CustomCommand{
		TriggerType: triggerType,
		Trigger:     trigger,
		Response:    response,
		ID:          int(id),
	}

	serialized, err := json.Marshal(cmd)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	redisCommands := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
		&common.RedisCmd{Name: "HSET", Args: []interface{}{"custom_commands:" + activeGuild.ID, cmdIndex, serialized}},
	}

	_, err = common.SafeRedisCommands(client, redisCommands)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed adding command", err))
		web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
		return
	}

	commands, _, err := GetCommands(client, activeGuild.ID)
	if err != nil {
		templateData.AddAlerts(web.ErrorAlert("Failed retrieving commands", err))
	} else {
		templateData["commands"] = commands
	}

	user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
	common.AddCPLogEntry(client, activeGuild.ID, fmt.Sprintf("%s(%s) Updated command #%s", user.Username, user.ID, cmdIndex))

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))
}

func HandleDeleteCommand(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(web.TemplateData)
	templateData["current_page"] = "custom_commands"

	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

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

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_custom_commands", templateData))

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
