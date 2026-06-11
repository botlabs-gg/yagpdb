package messagecreator

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/common/multiratelimit"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"goji.io"
	"goji.io/pat"
)

var messageRatelimiter = multiratelimit.NewMultiRatelimiter(1.0/60.0, 1)

type ratelimitKey struct{ guild, user int64 }

func allowAction(ctx context.Context, guildID int64) bool {
	var userID int64
	if u := web.ContextUser(ctx); u != nil {
		userID = u.ID
	}
	return messageRatelimiter.AllowN(ratelimitKey{guildID, userID}, time.Now(), 1)
}

//go:embed assets/messagecreator.html
var PageHTML string

var (
	panelLogKeySentMessage   = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "messagecreator_sent", FormatString: "Sent a message via Message Creator to channel %d"})
	panelLogKeyEditedMessage = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "messagecreator_edited", FormatString: "Edited message %d via Message Creator"})
)

var _ web.Plugin = (*Plugin)(nil)

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("messagecreator/assets/messagecreator.html", PageHTML)

	web.AddSidebarItem(web.SidebarCategoryTools, &web.SidebarItem{
		Name: "Message Creator",
		URL:  "messagecreator",
		Icon: "fas fa-paper-plane",
	})

	muxer := goji.SubMux()
	web.CPMux.Handle(pat.New("/messagecreator"), muxer)
	web.CPMux.Handle(pat.New("/messagecreator/*"), muxer)

	muxer.Use(web.RequireBotMemberMW)

	getHandler := web.ControllerHandler(handleGetCreator, "cp_messagecreator")
	muxer.Handle(pat.Get(""), getHandler)
	muxer.Handle(pat.Get("/"), getHandler)

	muxer.Handle(pat.Post("/send"), web.ControllerPostHandler(handleSend, getHandler, nil))
	muxer.Handle(pat.Post("/edit"), web.ControllerPostHandler(handleEdit, getHandler, nil))

	// /load returns raw JSON (consumed via fetch by the editor), so it bypasses template rendering.
	muxer.Handle(pat.Get("/load"), http.HandlerFunc(handleLoad))
}

func handleGetCreator(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	_, tmpl := web.GetBaseCPContextData(r.Context())
	return tmpl, nil
}

func handleSend(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	if err := r.ParseForm(); err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to parse form data")), nil
	}

	channelID, _ := strconv.ParseInt(r.FormValue("channel_id"), 10, 64)
	mode := r.FormValue("mode")
	payload := r.FormValue("payload")

	if activeGuild.GetChannelOrThread(channelID) == nil {
		return tmpl.AddAlerts(web.ErrorAlert("Please select a valid channel in this server")), nil
	}

	// Validate the payload early so the user gets a clear error before hitting the bot.
	if _, err := parsePayload(mode, []byte(payload)); err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Invalid message: " + err.Error())), nil
	}

	if !allowAction(ctx, activeGuild.ID) {
		return tmpl.AddAlerts(web.ErrorAlert("You're doing that too fast — you can send or edit at most one message per minute.")), nil
	}

	req := &SendRequest{ChannelID: channelID, Mode: mode, Payload: json.RawMessage(payload)}
	err := internalapi.PostWithGuild(activeGuild.ID, fmt.Sprintf("%d/messagecreator/send", activeGuild.ID), req, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to send message: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeySentMessage, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: channelID}))
	return tmpl.AddAlerts(web.SucessAlert("Message sent!")), nil
}

func handleEdit(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, tmpl := web.GetBaseCPContextData(ctx)

	if err := r.ParseForm(); err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to parse form data")), nil
	}

	mode := r.FormValue("mode")
	payload := r.FormValue("payload")

	guildID, channelID, messageID, err := parseMessageLink(r.FormValue("message_link"))
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert(err.Error())), nil
	}

	if guildID != activeGuild.ID {
		return tmpl.AddAlerts(web.ErrorAlert("That message link belongs to a different server")), nil
	}
	if activeGuild.GetChannelOrThread(channelID) == nil {
		return tmpl.AddAlerts(web.ErrorAlert("That message is not in a channel of this server")), nil
	}

	if _, err := parsePayload(mode, []byte(payload)); err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Invalid message: " + err.Error())), nil
	}

	if !allowAction(ctx, activeGuild.ID) {
		return tmpl.AddAlerts(web.ErrorAlert("You're doing that too fast — you can send or edit at most one message per minute.")), nil
	}

	req := &EditRequest{ChannelID: channelID, MessageID: messageID, Mode: mode, Payload: json.RawMessage(payload)}
	err = internalapi.PostWithGuild(activeGuild.ID, fmt.Sprintf("%d/messagecreator/edit", activeGuild.ID), req, nil)
	if err != nil {
		return tmpl.AddAlerts(web.ErrorAlert("Failed to edit message: " + err.Error())), nil
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(ctx, panelLogKeyEditedMessage, &cplogs.Param{Type: cplogs.ParamTypeInt, Value: messageID}))
	return tmpl.AddAlerts(web.SucessAlert("Message updated!")), nil
}

func handleLoad(w http.ResponseWriter, r *http.Request) {
	activeGuild, _ := web.GetBaseCPContextData(r.Context())

	writeErr := func(msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}

	guildID, channelID, messageID, err := parseMessageLink(r.URL.Query().Get("link"))
	if err != nil {
		writeErr(err.Error())
		return
	}
	if guildID != activeGuild.ID {
		writeErr("That message link belongs to a different server")
		return
	}
	if activeGuild.GetChannelOrThread(channelID) == nil {
		writeErr("That message is not in a channel of this server")
		return
	}

	var resp LoadResponse
	err = internalapi.GetWithGuild(activeGuild.ID, fmt.Sprintf("%d/messagecreator/message?channel=%d&message=%d", activeGuild.ID, channelID, messageID), &resp)
	if err != nil {
		writeErr(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
