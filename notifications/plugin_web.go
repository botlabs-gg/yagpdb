package notifications

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"golang.org/x/net/context"
	"log"
	"net/http"
)

func HandleNotificationsGet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	templateData["current_page"] = "notifications/general"
	templateData["current_config"] = GetConfig(client, activeGuild.ID)

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_notifications_general", templateData))
}

func HandleNotificationsPost(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client, activeGuild, templateData := web.GetBaseCPContextData(ctx)

	templateData["current_page"] = "notifications/general"

	previousConfig := GetConfig(client, activeGuild.ID)

	joinServer := r.FormValue("join_server_msg")
	joinDM := r.FormValue("join_dm_msg")
	leaveMsg := r.PostFormValue("leave_msg")

	// The new configuration
	newConfig := &Config{
		JoinServerEnabled: r.FormValue("join_server_enabled") == "on",
		JoinServerChannel: r.FormValue("join_server_channel"),
		JoinServerMsg:     joinServer,

		JoinDMEnabled: r.FormValue("join_dm_enabled") == "on",
		JoinDMMsg:     joinDM,

		LeaveEnabled: r.FormValue("leave_enabled") == "on",
		LeaveChannel: r.FormValue("leave_channel"),
		LeaveMsg:     leaveMsg,

		TopicEnabled: r.FormValue("topic_enabled") == "on",
		TopicChannel: r.FormValue("topic_channel"),

		PinEnabled: r.FormValue("pin_enabled") == "on",
		PinChannel: r.FormValue("pin_channel"),
	}

	// The sent one differs a little, we will send back invalid data but not store it
	sentConfig := *newConfig
	templateData["current_config"] = sentConfig

	foundErrors := false

	// Do some validation to make sure the user knows about faulty templates
	if joinServer != "" {
		_, err := ParseExecuteTemplate(joinServer, nil)
		if err != nil {
			templateData.AddAlerts(web.ErrorAlert("Failed parsing/executing template for server/channel join:", err))
			newConfig.JoinServerMsg = previousConfig.JoinServerMsg
			foundErrors = true
		}
	}

	if joinDM != "" {
		_, err := ParseExecuteTemplate(joinDM, nil)
		if err != nil {
			templateData.AddAlerts(web.ErrorAlert("Failed parsing/executing template for server/channel join:", err))
			newConfig.JoinDMMsg = previousConfig.JoinDMMsg
			foundErrors = true
		}
	}

	if leaveMsg != "" {
		_, err := ParseExecuteTemplate(leaveMsg, nil)
		if err != nil {
			templateData.AddAlerts(web.ErrorAlert("Failed parsing/executing template for server/channel join:", err))
			newConfig.LeaveMsg = previousConfig.LeaveMsg
			foundErrors = true
		}
	}

	if !foundErrors {
		templateData.AddAlerts(web.SucessAlert("Sucessfully saved everything! :')"))
	}

	r.ParseForm()

	serialized, err := json.Marshal(newConfig)
	if err == nil {
		user := ctx.Value(web.ContextKeyUser).(*discordgo.User)
		logMsg := fmt.Sprintf("%s(%s) updated notifications settings to %s", user.Username, user.ID, string(serialized))
		common.AddCPLogEntry(client, activeGuild.ID, logMsg)
	} else {
		log.Println("Failed serializing config", err)
	}

	err = common.SetRedisJson(client, "notifications/general:"+activeGuild.ID, newConfig)
	if err != nil {
		log.Println("Error setting config", err)
	}

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_notifications_general", templateData))
}
