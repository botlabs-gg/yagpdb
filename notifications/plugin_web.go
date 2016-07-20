package notifications

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
	"golang.org/x/net/context"
	"log"
	"net/http"
)

func HandleNotificationsGet(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(map[string]interface{})
	templateData["current_page"] = "notifications/general"

	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	templateData["current_config"] = GetConfig(client, activeGuild.ID)

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_notifications_general", templateData))
}

func HandleNotificationsPost(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := ctx.Value(web.ContextKeyTemplateData).(map[string]interface{})
	templateData["current_page"] = "notifications/general"

	client := web.RedisClientFromContext(ctx)
	activeGuild := ctx.Value(web.ContextKeyCurrentGuild).(*discordgo.Guild)

	templateData["current_config"] = GetConfig(client, activeGuild.ID)

	r.ParseForm()

	newConfig := &Config{
		JoinServerEnabled: r.FormValue("join_server_enabled") == "on",
		JoinServerChannel: r.FormValue("join_server_channel"),
		JoinServerMsg:     r.FormValue("join_server_msg"),

		JoinDMEnabled: r.FormValue("join_dm_enabled") == "on",
		JoinDMMsg:     r.FormValue("join_dm_msg"),

		LeaveEnabled: r.FormValue("leave_enabled") == "on",
		LeaveChannel: r.FormValue("leave_channel"),
		LeaveMsg:     r.PostFormValue("leave_msg"),

		TopicEnabled: r.FormValue("topic_enabled") == "on",
		TopicChannel: r.FormValue("topic_channel"),

		PinEnabled: r.FormValue("pin_enabled") == "on",
		PinChannel: r.FormValue("pin_channel"),
	}

	err := common.SetRedisJson(client, 0, "notifications/general:"+activeGuild.ID, newConfig)
	if err != nil {
		log.Println("Error setting config", err)
	} else {
		templateData["current_config"] = newConfig
	}

	web.LogIgnoreErr(web.Templates.ExecuteTemplate(w, "cp_notifications_general", templateData))
}
