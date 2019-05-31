package verification

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	seventsmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/verification/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/volatiletech/sqlboiler/boil"
	"math/rand"
	"strings"
	"time"
)

const InTicketPerms = discordgo.PermissionSendMessages | discordgo.PermissionReadMessages

var _ commands.CommandProvider = (*Plugin)(nil)
var _ bot.BotInitHandler = (*Plugin)(nil)

type VerificationEventData struct {
	UserID int64  `json:"user_id"`
	Token  string `json:"token"`
}

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p.handleMemberJoin, eventsystem.EventGuildMemberAdd)
	scheduledevents2.RegisterHandler("verification_user_verified", int64(0), ScheduledEventMW(p.handleUserVerifiedScheduledEvent))
	scheduledevents2.RegisterHandler("verification_user_warn", VerificationEventData{}, ScheduledEventMW(p.handleWarnUserVerification))
	scheduledevents2.RegisterHandler("verification_user_kick", VerificationEventData{}, ScheduledEventMW(p.handleKickUser))
}

func (p *Plugin) AddCommands() {

}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (p *Plugin) handleMemberJoin(evt *eventsystem.EventData) {
	m := evt.GuildMemberAdd()

	if m.User.Bot {
		return
	}

	conf, err := models.FindVerificationConfigG(context.Background(), m.GuildID)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.WithError(err).WithField("guild", m.GuildID).WithField("user", m.User.ID).Error("unable to retrieve config")
		}

		// either no config or an error occured
		return
	}

	if !conf.Enabled {
		return
	}

	go p.startVerificationProcess(conf, m.GuildID, m.User)
}

func (p *Plugin) createVerificationSession(userID, guildID int64) (string, error) {
	for {
		token := RandStringRunes(32)
		model := &models.VerificationSession{
			Token:     token,
			UserID:    userID,
			GuildID:   guildID,
			CreatedAt: time.Now(),
		}

		err := model.InsertG(context.Background(), boil.Infer())
		if err == nil {
			return token, nil
		}

		if common.ErrPQIsUniqueViolation(err) {
			// somehow we made a duplicate token...
			continue
		}

		// otherwise an unknown error occured
		return token, err
	}
}

func (p *Plugin) startVerificationProcess(conf *models.VerificationConfig, guildID int64, target *discordgo.User) {

	token, err := p.createVerificationSession(target.ID, guildID)
	if err != nil {
		logger.WithError(err).WithField("user", target.ID).WithField("guild", guildID).Error("failed creating verification session")
		return
	}

	gs := bot.State.Guild(true, guildID)
	if gs == nil {
		logger.Error("guild not available")
		return
	}

	msg := conf.DMMessage
	if strings.TrimSpace(msg) == "" {
		msg = DefaultDMMessage
	}

	ms, err := bot.GetMember(guildID, target.ID)
	if err != nil {
		logger.WithError(err).Error("failed retrieving member")
		return
	}

	channel, err := common.BotSession.UserChannelCreate(ms.ID)
	if err != nil {
		logger.WithError(err).Error("failed creating user channel")
		return
	}

	tmplCTX := templates.NewContext(gs, dstate.NewChannelState(gs, gs, channel), ms)
	tmplCTX.Name = "dm_veification_message"
	tmplCTX.Data["Link"] = fmt.Sprintf("%s/public/%d/verify/%d/%s", web.BaseURL(), guildID, target.ID, token)

	err = tmplCTX.ExecuteAndSendWithErrors(msg, channel.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).WithField("user", ms.ID).Error("failed sending verification dm message")
	}

	evt := &VerificationEventData{
		UserID: target.ID,
		Token:  token,
	}

	// schedule the kick and warnings
	if conf.WarnUnverifiedAfter > 0 && conf.WarnMessage != "" {
		scheduledevents2.ScheduleEvent("verification_user_warn", guildID, time.Now().Add(time.Minute*time.Duration(conf.WarnUnverifiedAfter)), evt)
	}
	if conf.KickUnverifiedAfter > 0 {
		scheduledevents2.ScheduleEvent("verification_user_kick", guildID, time.Now().Add(time.Minute*time.Duration(conf.KickUnverifiedAfter)), evt)
	}

	p.logAction(conf.LogChannel, target, "New user joined waiting to be verified as a human", 0x47aaed)
}

func ScheduledEventMW(innerHandler func(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig, rawData interface{}) (bool, error)) func(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	return func(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {

		userID := int64(0)

		switch t := data.(type) {
		case *int64:
			userID = *t
		case *VerificationEventData:
			userID = t.UserID
		}

		conf, err := models.FindVerificationConfigG(context.Background(), evt.GuildID)
		if err != nil {
			if err != sql.ErrNoRows {
				logger.WithError(err).WithField("guild", evt.GuildID).WithField("user", userID).Error("unable to retrieve config")
				return true, err
			}

			// either no config anymore? shouldn't be possible
			return false, nil
		}

		gs := bot.State.Guild(true, evt.GuildID)
		if gs == nil {
			return false, nil
		}

		ms := gs.MemberCopy(true, userID)
		if ms == nil {

			if gs.IsAvailable(true) {
				return false, nil // probably left
			}

			// unavailable, we might be starting up
			return true, nil
		}

		return innerHandler(ms, evt.GuildID, conf, data)
	}

}

func (p *Plugin) handleUserVerifiedScheduledEvent(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig, rawData interface{}) (retry bool, err error) {

	err = common.BotSession.GuildMemberRoleAdd(guildID, ms.ID, conf.VerifiedRole)
	if err == nil {
		p.logAction(conf.LogChannel, ms.DGoUser(), "User successfully verified", 0x49ed47)
	}

	return scheduledevents2.CheckDiscordErrRetry(err), err
}

func (p *Plugin) handleWarnUserVerification(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig, rawData interface{}) (retry bool, err error) {
	gs := bot.State.Guild(true, guildID)
	if gs == nil {
		return false, nil
	}

	d := rawData.(*VerificationEventData)

	if !common.ContainsInt64Slice(ms.Roles, conf.VerifiedRole) {
		err := p.sendWarning(ms, gs, d.Token, conf)
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	// verified
	return false, nil
}

func (p *Plugin) sendWarning(ms *dstate.MemberState, gs *dstate.GuildState, token string, conf *models.VerificationConfig) error {

	msg := conf.WarnMessage
	if strings.TrimSpace(msg) == "" {
		return nil // no message to send
	}

	channel, err := common.BotSession.UserChannelCreate(ms.ID)
	if err != nil {
		return err
	}

	tmplCTX := templates.NewContext(gs, dstate.NewChannelState(gs, gs, channel), ms)
	tmplCTX.Name = "warn message"
	tmplCTX.Data["Link"] = fmt.Sprintf("%s/public/%d/verify/%d/%s", web.BaseURL(), gs.ID, ms.ID, token)

	err = tmplCTX.ExecuteAndSendWithErrors(msg, channel.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).WithField("user", ms.ID).Error("failed sending warning message")
	}

	return nil
}

func (p *Plugin) handleKickUser(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig, rawData interface{}) (retry bool, err error) {
	if !common.ContainsInt64Slice(ms.Roles, conf.VerifiedRole) {
		err := common.BotSession.GuildMemberDelete(guildID, ms.ID)
		if err == nil {
			p.logAction(conf.LogChannel, ms.DGoUser(), "Kicked for not verifying within deadline", 0xef4640)
		}

		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	// verified
	return false, nil
}

func (p *Plugin) logAction(channelID int64, author *discordgo.User, action string, color int) {
	if channelID == 0 {
		return
	}

	_, err := common.BotSession.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			IconURL: author.AvatarURL("128"),
			Name:    fmt.Sprintf("%s#%s (%d)", author.Username, author.Discriminator, author.ID),
		},
		Description: action,
		Color:       color,
	})

	if err != nil {
		logger.WithError(err).WithField("channel", channelID).Error("failed sending log message")
	}
}
