package tickets

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/tickets/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLast(p, p.handleChannelRemoved, eventsystem.EventChannelDelete)
	eventsystem.AddHandlerAsyncLast(p, p.handleInteractionCreate, eventsystem.EventInteractionCreate)
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
	ErrNoTicketCategory    TicketUserError = "No category for ticket channels set"
	ErrTitleTooLong        TicketUserError = "Title is too long (max 90 characters.) Please shorten it down, you can add more details in the ticket after it has been created"
	ErrMaxOpenTickets      TicketUserError = "You're currently in over 10 open tickets on this server, please close some of the ones you're in."
	ErrMaxCategoryChannels TicketUserError = "Max channels in category reached (50)"
)

const (
	AppendButtonsClose           int64 = 1 << 0
	AppendButtonsCloseWithReason int64 = 1 << 1
)

func CreateTicket(ctx context.Context, gs *dstate.GuildSet, ms *dstate.MemberState, conf *models.TicketConfig, topic string, checkMaxTickets, executedByCommandTemplate bool) (*dstate.GuildSet, *models.Ticket, error) {
	if gs.GetChannel(conf.TicketsChannelCategory) == nil {
		return gs, nil, ErrNoTicketCategory
	}

	categoryChannels := 0
	for _, v := range gs.Channels {
		if v.ParentID == conf.TicketsChannelCategory {
			categoryChannels++
		}

		if categoryChannels == 50 {
			return gs, nil, ErrMaxCategoryChannels
		}
	}

	if hasPerms, _ := bot.BotHasPermissionGS(gs, conf.TicketsChannelCategory, InTicketPerms); !hasPerms {
		return gs, nil, TicketUserError(fmt.Sprintf("The bot is missing one of the following permissions on the ticket category: %s", common.HumanizePermissions(InTicketPerms)))
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
	if executedByCommandTemplate {
		tmplCTX.ExecutedFrom = templates.ExecutedFromNestedCommandTemplate
	} else {
		tmplCTX.ExecutedFrom = templates.ExecutedFromCommandTemplate
	}
	tmplCTX.Name = "ticket open message"
	tmplCTX.Data["Reason"] = topic
	ticketOpenMsg := conf.TicketOpenMSG
	if ticketOpenMsg == "" {
		ticketOpenMsg = DefaultTicketMsg
	}

	if conf.AppendButtons&AppendButtonsClose == AppendButtonsClose {
		tmplCTX.CurrentFrame.ComponentsToSend = append(tmplCTX.CurrentFrame.ComponentsToSend, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{
			Label:    "Close Ticket",
			CustomID: "tickets-close",
			Style:    discordgo.DangerButton,
		}}})
	}
	if conf.AppendButtons&AppendButtonsCloseWithReason == AppendButtonsCloseWithReason {
		tmplCTX.CurrentFrame.ComponentsToSend = append(tmplCTX.CurrentFrame.ComponentsToSend, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{
			Label:    "Close Ticket with Reason",
			CustomID: "tickets-close-reason",
			Style:    discordgo.SecondaryButton,
		}}})
	}

	err = tmplCTX.ExecuteAndSendWithErrors(ticketOpenMsg, channel.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Error("failed sending ticket open message")
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

func openTicket(ctx context.Context, gs *dstate.GuildSet, ms *dstate.MemberState, conf *models.TicketConfig, reason string) (string, error) {
	_, ticket, err := CreateTicket(ctx, gs, ms, conf, reason, true, ctx.Value(commands.CtxKeyExecutedByCommandTemplate) == true)
	if err != nil {
		switch t := err.(type) {
		case TicketUserError:
			return string(t), nil
		case *TicketUserError:
			return string(*t), nil
		}

		return "", err
	}

	// Annn done setting up the ticket
	return fmt.Sprintf("Ticket #%d opened in <#%d>", ticket.LocalID, ticket.ChannelID), nil
}

var closingTickets = make(map[int64]bool)
var closingTicketsLock sync.Mutex

const closingTicketMsg = "Closing ticket, creating logs, downloading attachments and so on.\nThis may take a while if the ticket is big."

func closeTicket(gs *dstate.GuildSet, currentTicket *Ticket, ticketCS *dstate.ChannelState, conf *models.TicketConfig, member *discordgo.User, reason string, ctx context.Context) (string, error) {
	// protect again'st calling close multiple times at the sime time
	closingTicketsLock.Lock()
	if _, ok := closingTickets[currentTicket.Ticket.ChannelID]; ok {
		closingTicketsLock.Unlock()
		return "Already working on closing this ticket, please wait...", nil
	}
	closingTickets[currentTicket.Ticket.ChannelID] = true
	closingTicketsLock.Unlock()
	defer func() {
		closingTicketsLock.Lock()
		delete(closingTickets, currentTicket.Ticket.ChannelID)
		closingTicketsLock.Unlock()
	}()

	// send a heads up that this can take a while
	common.BotSession.ChannelMessageSend(currentTicket.Ticket.ChannelID, closingTicketMsg)

	currentTicket.Ticket.ClosedAt.Time = time.Now()
	currentTicket.Ticket.ClosedAt.Valid = true

	isAdminsOnly := ticketIsAdminOnly(conf, ticketCS)

	// create the logs, download the attachments
	err := createLogs(gs, conf, currentTicket.Ticket, isAdminsOnly)
	if err != nil {
		return "Cannot send transcript to ticket logs channel, refusing to close ticket.", err
	}

	TicketLog(conf, gs.ID, member, &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Ticket #%d - '%s' closed", currentTicket.Ticket.LocalID, currentTicket.Ticket.Title),
		Description: fmt.Sprintf("Reason: %s", reason),
		Color:       0xf23c3c,
	})

	// if everything went well, delete the channel
	_, err = common.BotSession.ChannelDelete(currentTicket.Ticket.ChannelID)
	if err != nil {
		return "", err
	}

	_, err = currentTicket.Ticket.UpdateG(ctx, boil.Whitelist("closed_at"))
	if err != nil {
		return "", err
	}

	return "", nil
}

func handleButton(evt *eventsystem.EventData, ic *discordgo.InteractionCreate, member *discordgo.Member, conf *models.TicketConfig, currentChannel *dstate.ChannelState) (*discordgo.InteractionResponse, error) {
	interaction := ic.MessageComponentData()
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}

	var err error
	cID := strings.TrimPrefix(interaction.CustomID, "tickets-")
	if cID, ok := strings.CutPrefix(cID, "open-"); ok {
		if cID == "" {
			response = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					Title:    "Create a Ticket",
					CustomID: "tickets-open-modal",
					Components: []discordgo.MessageComponent{discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{discordgo.TextInput{
							CustomID:  "reason",
							Label:     "Reason for opening",
							Style:     discordgo.TextInputShort,
							Required:  true,
							MaxLength: 90,
						}},
					}},
				},
			}
		} else {
			common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})

			response.Data.Content, err = openTicket(evt.Context(), evt.GS, dstate.MemberStateFromMember(member), conf, cID)
		}
		return response, err
	}

	switch cID {
	case "close":
		activeTicket, err := models.Tickets(qm.Where("channel_id = ? AND guild_id = ?", currentChannel.ID, evt.GS.ID)).OneG(evt.Context())
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if activeTicket == nil {
			response.Data.Content = "A problem occured, failed to close the ticket."
			return response, err
		}

		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		var currentTicket *Ticket
		participants, _ := models.TicketParticipants(qm.Where("ticket_guild_id = ? AND ticket_local_id = ?", activeTicket.GuildID, activeTicket.LocalID)).AllG(evt.Context())
		currentTicket = &Ticket{
			Ticket:       activeTicket,
			Participants: participants,
		}

		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: closingTicketMsg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		response.Data.Content, err = closeTicket(evt.GS, currentTicket, currentChannel, conf, member.User, "", evt.Context())
	case "close-reason":
		response = &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				Title:    "Close Ticket",
				CustomID: "tickets-close-modal",
				Components: []discordgo.MessageComponent{discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{discordgo.TextInput{
						CustomID:  "reason",
						Label:     "Reason for closing",
						Style:     discordgo.TextInputShort,
						Required:  true,
						MaxLength: 90,
					}},
				}},
			},
		}
	}

	return response, err
}

func handleModal(evt *eventsystem.EventData, ic *discordgo.InteractionCreate, member *discordgo.Member, conf *models.TicketConfig, currentChannel *dstate.ChannelState) (*discordgo.InteractionResponse, error) {
	interaction := ic.ModalSubmitData()
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	var err error
	value := interaction.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	switch {
	case strings.Contains(interaction.CustomID, "open"):
		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

		response.Data.Content, err = openTicket(evt.Context(), evt.GS, dstate.MemberStateFromMember(member), conf, value)
	case strings.Contains(interaction.CustomID, "close"):
		activeTicket, err := models.Tickets(qm.Where("channel_id = ? AND guild_id = ?", currentChannel.ID, evt.GS.ID)).OneG(evt.Context())
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if activeTicket == nil {
			response.Data.Content = "A problem occured, failed to close the ticket."
			return response, err
		}

		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		var currentTicket *Ticket
		participants, _ := models.TicketParticipants(qm.Where("ticket_guild_id = ? AND ticket_local_id = ?", activeTicket.GuildID, activeTicket.LocalID)).AllG(evt.Context())
		currentTicket = &Ticket{
			Ticket:       activeTicket,
			Participants: participants,
		}

		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: closingTicketMsg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		response.Data.Content, err = closeTicket(evt.GS, currentTicket, currentChannel, conf, member.User, value, evt.Context())
	}

	return response, err
}

func (p *Plugin) handleInteractionCreate(evt *eventsystem.EventData) (retry bool, err error) {
	ic := evt.InteractionCreate()

	if ic.GuildID == 0 {
		// ignore dm interactions
		return
	}

	var customID string
	switch ic.Type {
	case discordgo.InteractionMessageComponent:
		customID = ic.MessageComponentData().CustomID
	case discordgo.InteractionModalSubmit:
		customID = ic.ModalSubmitData().CustomID
	default:
		return
	}

	// continue only if this component is for tickets
	if ticketCID := strings.HasPrefix(customID, "tickets-"); !ticketCID {
		return
	}

	evt.GS = bot.State.GetGuild(ic.GuildID)
	conf, err := models.FindTicketConfigG(evt.Context(), ic.GuildID)
	if err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}

		conf = &models.TicketConfig{}
	}

	if !conf.Enabled && strings.Contains(customID, "open") {
		common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: createTicketsDisabledError(ic.GuildID),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var response *discordgo.InteractionResponse
	errorResponse := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Something went wrong when running this ticket interaction, either discord or the bot may be having issues.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}

	var currentChannel *dstate.ChannelState = evt.GS.GetChannel(ic.ChannelID)

	switch ic.Type {
	case discordgo.InteractionMessageComponent:
		response, err = handleButton(evt, ic, ic.Member, conf, currentChannel)
	case discordgo.InteractionModalSubmit:
		response, err = handleModal(evt, ic, ic.Member, conf, currentChannel)
	}
	if response != nil {
		if response.Data.Content == "" && len(response.Data.Components) == 0 {
			response = errorResponse
		}
	} else {
		response = errorResponse
	}

	respErr := common.BotSession.CreateInteractionResponse(ic.ID, ic.Token, response)
	if respErr != nil {
		// try again as a followup, if that still fails, return the original error
		_, followupErr := common.BotSession.CreateFollowupMessage(ic.ApplicationID, ic.Token, &discordgo.WebhookParams{
			Content: response.Data.Content,
			Flags:   int64(response.Data.Flags),
		})
		if followupErr != nil {
			return bot.CheckDiscordErrRetry(respErr), respErr
		}
	}
	return false, err
}
