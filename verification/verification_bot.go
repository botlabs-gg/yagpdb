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
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

const InTicketPerms = discordgo.PermissionSendMessages | discordgo.PermissionReadMessages

var _ commands.CommandProvider = (*Plugin)(nil)
var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(p.handleMemberJoin, eventsystem.EventGuildMemberAdd)
	scheduledevents2.RegisterHandler("verification_user_verified", int64(0), ScheduledEventMW(p.handleUserVerifiedScheduledEvent))
	scheduledevents2.RegisterHandler("verification_user_warn", int64(0), ScheduledEventMW(p.handleWarnUserVerification))
	scheduledevents2.RegisterHandler("verification_user_kick", int64(0), ScheduledEventMW(p.handleKickUser))
}

func (p *Plugin) AddCommands() {

}

func (p *Plugin) handleMemberJoin(evt *eventsystem.EventData) {
	m := evt.GuildMemberAdd()

	conf, err := models.FindVerificationConfigG(context.Background(), m.GuildID)
	if err != nil {
		if err != sql.ErrNoRows {
			logrus.WithError(err).WithField("guild", m.GuildID).WithField("user", m.User.ID).Error("[verification] unable to retrieve config")
		}

		// either no config or an error occured
		return
	}

	if !conf.Enabled {
		return
	}

	go p.startVerificationProcess(conf, m.GuildID, m.User)
}

func (p *Plugin) startVerificationProcess(conf *models.VerificationConfig, guildID int64, target *discordgo.User) {
	// TODO Make configurable
	const msgF = "Please verify that you're not a bot by going through google reCAPTCHA on this link: %s/public/%d/verify/%d"

	gcop := bot.State.LightGuildCopy(true, guildID)

	err := bot.SendDMEmbed(target.ID, &discordgo.MessageEmbed{
		Title:       "Make sure you're not a bot",
		Description: fmt.Sprintf(msgF, web.BaseURL(), guildID, target.ID),

		Author: &discordgo.MessageEmbedAuthor{
			IconURL: discordgo.EndpointGuildIcon(gcop.ID, gcop.Icon),
			Name:    gcop.Name,
		},
	})

	if err != nil {
		// TODO log somewhere
		logrus.WithError(err).WithField("guild", guildID).WithField("user", target.ID).Error("[verification] failed sending verify message to user")
	}

	// schedule the kick and warnings
	if conf.WarnUnverifiedAfter > 0 && conf.WarnMessage != "" {
		scheduledevents2.ScheduleEvent("verification_user_warn", guildID, time.Now().Add(time.Minute*time.Duration(conf.WarnUnverifiedAfter)), target.ID)
	}
	if conf.KickUnverifiedAfter > 0 {
		scheduledevents2.ScheduleEvent("verification_user_kick", guildID, time.Now().Add(time.Minute*time.Duration(conf.KickUnverifiedAfter)), target.ID)
	}

	p.logAction(conf.LogChannel, target, "New user joined waiting to be verified as a human", 0x47aaed)
}

func ScheduledEventMW(innerHandler func(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig) (bool, error)) func(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	return func(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {

		userID := *data.(*int64)

		conf, err := models.FindVerificationConfigG(context.Background(), evt.GuildID)
		if err != nil {
			if err != sql.ErrNoRows {
				logrus.WithError(err).WithField("guild", evt.GuildID).WithField("user", userID).Error("[verification] unable to retrieve config")
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

		return innerHandler(ms, evt.GuildID, conf)
	}

}

func (p *Plugin) handleUserVerifiedScheduledEvent(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig) (retry bool, err error) {

	err = common.BotSession.GuildMemberRoleAdd(guildID, ms.ID, conf.VerifiedRole)
	if err == nil {
		p.logAction(conf.LogChannel, ms.DGoUser(), "User sucessfully verified", 0x49ed47)
	}

	return scheduledevents2.CheckDiscordErrRetry(err), err
}

func (p *Plugin) handleWarnUserVerification(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig) (retry bool, err error) {
	gs := bot.State.Guild(true, guildID)
	if gs == nil {
		return false, nil
	}

	if !common.ContainsInt64Slice(ms.Roles, conf.VerifiedRole) {
		err := p.sendWarning(ms, gs, conf)
		return scheduledevents2.CheckDiscordErrRetry(err), err
	}

	// verified
	return false, nil
}

func (p *Plugin) sendWarning(ms *dstate.MemberState, gs *dstate.GuildState, conf *models.VerificationConfig) error {

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

	err = tmplCTX.ExecuteAndSendWithErrors(msg, channel.ID)
	if err != nil {
		logrus.WithError(err).WithField("guild", gs).WithField("user", ms.ID).Error("[verification] failed sending warning message")
	}

	return nil
}

func (p *Plugin) handleKickUser(ms *dstate.MemberState, guildID int64, conf *models.VerificationConfig) (retry bool, err error) {
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
		logrus.WithError(err).WithField("channel", channelID).Error("[verification] failed sending log message")
	}
}
