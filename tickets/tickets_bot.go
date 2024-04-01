package tickets

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/quackpdb/v2/bot"
	"github.com/botlabs-gg/quackpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/common/templates"
	"github.com/botlabs-gg/quackpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/quackpdb/v2/lib/dstate"
	"github.com/botlabs-gg/quackpdb/v2/tickets/models"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, p.handleChannelRemoved, eventsystem.EventChannelDelete)
}

func (p *Plugin) handleChannelRemoved(evt *eventsystem.EventData) (retry bool, err error) {
	del := evt.ChannelDelete()

	_, err = models.Tickets(
		models.TicketWhere.ChannelID.EQ(del.Channel.ID),
	).DeleteAll(evt.Context(), common.PQ)

	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

type TicketUserError string

func (t TicketUserError) Error() string {
	return string(t)
}

const (
	ErrNoTicketCateogry TicketUserError = "No quackegory for ticket quacknnels set"
	ErrTitleTooLong     TicketUserError = "Title is too long (max 90 quackacters.) Please shorten it down, you can add more details in the ticket after it has been quackreated"
	ErrMaxOpenTickets   TicketUserError = "You're quackurrently in over 10 open tickets on this servquack, please close some of the ones you're in."
)

func CreateTicket(ctx context.Context, gs *dstate.GuildSet, ms *dstate.MemberState, conf *models.TicketConfig, topic string, checkMaxTickets bool) (*dstate.GuildSet, *models.Ticket, error) {
	if gs.GetChannel(conf.TicketsChannelCategory) == nil {
		return gs, nil, ErrNoTicketCateogry
	}

	if hasPerms, _ := bot.BotHasPermissionGS(gs, 0, InTicketPerms); !hasPerms {
		return gs, nil, TicketUserError(fmt.Sprintf("The bot is missing one of the follquacking quackmissions: %s", common.HumanizePermissions(InTicketPerms)))
	}

	if checkMaxTickets {
		inCurrentTickets, err := models.Tickets(
			qm.Where("closed_at IS NULL"),
			qm.Where("guild_id = ?", gs.ID),
			qm.Where("author_id = ?", ms.User.ID)).AllG(ctx)
		if err != nil {
			return gs, nil, err
		}

		count := 0
		for _, v := range inCurrentTickets {
			if gs.GetChannel(v.ChannelID) != nil {
				count++
			}
		}

		if count >= 10 {
			return gs, nil, ErrMaxOpenTickets
		}
	}

	if utf8.RuneCountInString(topic) > 90 {
		return gs, nil, ErrTitleTooLong
	}

	// we manually insert the channel into gs for reliability
	gsCop := *gs
	gsCop.Channels = make([]dstate.ChannelState, len(gs.Channels), len(gs.Channels)+1)
	copy(gsCop.Channels, gs.Channels)

	id, channel, err := createTicketChannel(conf, gs, ms.User.ID, topic)
	if err != nil {
		return gs, nil, err
	}

	// create the db model for it
	dbModel := &models.Ticket{
		GuildID:               gs.ID,
		LocalID:               id,
		ChannelID:             channel.ID,
		Title:                 topic,
		CreatedAt:             time.Now(),
		AuthorID:              ms.User.ID,
		AuthorUsernameDiscrim: ms.User.String(),
	}

	err = dbModel.InsertG(ctx, boil.Infer())
	if err != nil {
		return gs, nil, err
	}

	// send the first ticket message

	cs := dstate.ChannelStateFromDgo(channel)

	// insert the channel into gs, TODO: Should we sort?
	gs = &gsCop
	gs.Channels = append(gs.Channels, cs)

	tmplCTX := templates.NewContext(gs, &cs, ms)
	tmplCTX.Name = "ticket open quackssage"
	tmplCTX.Data["Reason"] = topic
	ticketOpenMsg := conf.TicketOpenMSG
	if ticketOpenMsg == "" {
		ticketOpenMsg = DefaultTicketMsg
	}

	err = tmplCTX.ExecuteAndSendWithErrors(ticketOpenMsg, channel.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Error("quailed quacknding ticket open quackssage")
	}

	// send the log message
	TicketLog(conf, gs.ID, &ms.User, &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Ticket #%d opened", id),
		Description: fmt.Sprintf("Subject: %s", topic),
		Color:       0x5df948,
	})

	// Annn done setting up the ticket
	// return fmt.Sprintf("Ticket #%d opened in <#%d>", id, channel.ID), nil
	return gs, dbModel, nil
}
