package messagecreator

import (
	"encoding/json"
	"net/http"
	"strconv"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"goji.io"
	"goji.io/pat"
)

var _ internalapi.InternalAPIPlugin = (*Plugin)(nil)

func (p *Plugin) InitInternalAPIRoutes(mux *goji.Mux) {
	if !bot.Enabled {
		return
	}
	mux.Handle(pat.Post("/:guild/messagecreator/send"), http.HandlerFunc(handleBotSend))
	mux.Handle(pat.Post("/:guild/messagecreator/edit"), http.HandlerFunc(handleBotEdit))
	mux.Handle(pat.Get("/:guild/messagecreator/message"), http.HandlerFunc(handleBotGetMessage))
}

func channelInGuild(guildID, channelID int64) bool {
	guild := bot.State.GetGuild(guildID)
	if guild == nil {
		return false
	}
	return guild.GetChannelOrThread(channelID) != nil
}

func handleBotSend(w http.ResponseWriter, r *http.Request) {
	guildID, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	if guildID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to decode request"))
		return
	}

	if !channelInGuild(guildID, req.ChannelID) {
		internalapi.ServerError(w, r, errors.New("channel does not belong to this server"))
		return
	}

	msg, err := parsePayload(req.Mode, req.Payload)
	if err != nil {
		internalapi.ServerError(w, r, err)
		return
	}

	sent, err := common.BotSession.ChannelMessageSendComplex(req.ChannelID, msg)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to send message"))
		return
	}

	internalapi.ServeJson(w, r, sent.ID)
}

func handleBotEdit(w http.ResponseWriter, r *http.Request) {
	guildID, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	if guildID == 0 {
		internalapi.ServerError(w, r, errors.New("unknown server"))
		return
	}

	var req EditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to decode request"))
		return
	}

	if !channelInGuild(guildID, req.ChannelID) {
		internalapi.ServerError(w, r, errors.New("channel does not belong to this server"))
		return
	}

	existing, err := common.BotSession.ChannelMessage(req.ChannelID, req.MessageID)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to fetch message"))
		return
	}
	if existing.Author == nil || existing.Author.ID != common.BotUser.ID {
		internalapi.ServerError(w, r, errors.New("you must select a message that YAGPDB has sent"))
		return
	}

	msg, err := parsePayload(req.Mode, req.Payload)
	if err != nil {
		internalapi.ServerError(w, r, err)
		return
	}

	_, err = common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Content:    &msg.Content,
		Embeds:     msg.Embeds,
		Components: msg.Components,
		Flags:      msg.Flags,
		ID:         req.MessageID,
		Channel:    req.ChannelID,
	})
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to edit message"))
		return
	}

	internalapi.ServeJson(w, r, "ok")
}

func handleBotGetMessage(w http.ResponseWriter, r *http.Request) {
	guildID, _ := strconv.ParseInt(pat.Param(r, "guild"), 10, 64)
	channelID, _ := strconv.ParseInt(r.URL.Query().Get("channel"), 10, 64)
	messageID, _ := strconv.ParseInt(r.URL.Query().Get("message"), 10, 64)

	if !channelInGuild(guildID, channelID) {
		internalapi.ServerError(w, r, errors.New("channel does not belong to this server"))
		return
	}

	existing, err := common.BotSession.ChannelMessage(channelID, messageID)
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to fetch message"))
		return
	}

	mode := ModeNormal
	if existing.Flags&v2Flag != 0 {
		mode = ModeComponentsV2
	}

	// Reconstruct an editor payload from the fetched message.
	payload, err := json.Marshal(&discordgo.MessageSend{
		Content:    existing.Content,
		Embeds:     existing.Embeds,
		Components: existing.Components,
	})
	if err != nil {
		internalapi.ServerError(w, r, errors.WithMessage(err, "failed to encode message"))
		return
	}

	internalapi.ServeJson(w, r, LoadResponse{
		AuthorIsBot: existing.Author != nil && existing.Author.ID == common.BotUser.ID,
		Mode:        mode,
		Payload:     payload,
	})
}
