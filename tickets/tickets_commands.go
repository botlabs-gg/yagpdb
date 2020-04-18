package tickets

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/tickets/models"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

const InTicketPerms = discordgo.PermissionReadMessageHistory | discordgo.PermissionReadMessages | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks | discordgo.PermissionAttachFiles

var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {

	categoryTickets := &dcmd.Category{
		Name:        "Tickets",
		Description: "Ticket commands",
		HelpEmoji:   "🎫",
		EmbedColor:  0x42b9f4,
	}

	cmdOpenTicket := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "Open",
		Aliases:      []string{"create", "new", "make"},
		Description:  "Opens a new ticket",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "subject", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)
			if !conf.Enabled {
				return "Ticket system is disabled in this server, the server admins can enable it in the control panel.", nil
			}

			if parsed.GS.Channel(true, conf.TicketsChannelCategory) == nil {
				return "No category for ticket channels set", nil
			}

			inCurrentTickets, err := models.Tickets(
				qm.Where("closed_at IS NULL"),
				qm.Where("guild_id = ?", parsed.GS.ID),
				qm.Where("author_id = ?", parsed.Msg.Author.ID)).AllG(parsed.Context())

			count := 0
			for _, v := range inCurrentTickets {
				if parsed.GS.Channel(true, v.ChannelID) != nil {
					count++
				}
			}

			if count >= 10 {
				return "You're currently in over 10 open tickets on this server, please close some of the ones you're in.", nil
			}

			if len(parsed.Args[0].Str()) > 90 {
				return "Title is too long (max 90 characters.) Please shorten it down, you can add more details in the ticket after it has been created", nil
			}

			subject := parsed.Args[0].Str()
			id, channel, err := createTicketChannel(conf, parsed.GS, parsed.Msg.Author.ID, subject)
			if err != nil {
				return "Failed creating the channel, make sure the bot has proper perms and the channel limit hasn't been reached.", nil
			}

			// create the db model for it
			dbModel := &models.Ticket{
				GuildID:               parsed.GS.ID,
				LocalID:               id,
				ChannelID:             channel.ID,
				Title:                 subject,
				CreatedAt:             time.Now(),
				AuthorID:              parsed.Msg.Author.ID,
				AuthorUsernameDiscrim: parsed.Msg.Author.Username + "#" + parsed.Msg.Author.Discriminator,
			}

			err = dbModel.InsertG(parsed.Context(), boil.Infer())
			if err != nil {
				return nil, err
			}

			// send the first ticket message

			tmplCTX := templates.NewContext(parsed.GS, dstate.NewChannelState(parsed.GS, parsed.GS, channel), parsed.MS)
			tmplCTX.Name = "ticket open message"
			tmplCTX.Data["Reason"] = parsed.Args[0].Str()
			ticketOpenMsg := conf.TicketOpenMSG
			if ticketOpenMsg == "" {
				ticketOpenMsg = DefaultTicketMsg
			}

			err = tmplCTX.ExecuteAndSendWithErrors(ticketOpenMsg, channel.ID)
			if err != nil {
				logger.WithError(err).WithField("guild", parsed.GS.ID).Error("failed sending ticket open message")
			}

			// send the log message
			TicketLog(conf, parsed.GS.ID, parsed.Msg.Author, &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("Ticket #%d opened", id),
				Description: fmt.Sprintf("Subject: %s", subject),
				Color:       0x5df948,
			})

			// Annn done setting up the ticket
			return fmt.Sprintf("Ticket #%d opened in <#%d>", id, channel.ID), nil
		},
	}

	cmdAddParticipant := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "AddUser",
		Description:  "Adds a user to the ticket in this channel",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "target", Type: &commands.MemberArg{}},
		},

		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Args[0].Value.(*dstate.MemberState)

			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

			parsed.GS.RLock()
		OUTER:
			for _, v := range parsed.CS.PermissionOverwrites {
				if v.Type == "member" && v.ID == target.ID {
					if (v.Allow & InTicketPerms) == InTicketPerms {
						parsed.GS.RUnlock()
						return "User is already part of the ticket", nil
					}

					break OUTER
				}
			}
			parsed.GS.RUnlock()

			err := common.BotSession.ChannelPermissionSet(currentTicket.Ticket.ChannelID, target.ID, "member", InTicketPerms, 0)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Added %s#%04d to the ticket", target.Username, target.Discriminator), nil
		},
	}

	cmdRemoveParticipant := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "RemoveUser",
		Description:  "Removes a user from the ticket",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "target", Type: &commands.MemberArg{}},
		},

		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Args[0].Value.(*dstate.MemberState)

			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

			foundUser := false

			parsed.GS.RLock()
		OUTER:
			for _, v := range parsed.CS.PermissionOverwrites {
				if v.Type == "member" && v.ID == target.ID {
					if (v.Allow & InTicketPerms) == InTicketPerms {
						foundUser = true
					}

					break OUTER
				}
			}
			parsed.GS.RUnlock()

			if !foundUser {
				return fmt.Sprintf("%s#%04d is already not (explicitly) part of this ticket", target.Username, target.Discriminator), nil
			}

			err := common.BotSession.ChannelPermissionDelete(currentTicket.Ticket.ChannelID, target.ID)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Removed %s#%04d from the ticket", target.Username, target.Discriminator), nil
		},
	}

	cmdRenameTicket := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "Rename",
		Description:  "Renames the ticket",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "new-name", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

			newName := parsed.Args[0].Str()

			oldName := currentTicket.Ticket.Title
			currentTicket.Ticket.Title = newName
			_, err := currentTicket.Ticket.UpdateG(parsed.Context(), boil.Whitelist("title"))
			if err != nil {
				return nil, err
			}

			_, err = common.BotSession.ChannelEdit(currentTicket.Ticket.ChannelID, fmt.Sprintf("#%d-%s", currentTicket.Ticket.LocalID, newName))
			if err != nil {
				return nil, err
			}

			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)
			TicketLog(conf, parsed.GS.ID, parsed.Msg.Author, &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("Ticket #%d renamed", currentTicket.Ticket.LocalID),
				Description: fmt.Sprintf("From %q to %q", oldName, newName),
				Color:       0x5394fc,
			})

			return "Renamed ticket to " + newName, nil
		},
	}

	closingTickets := make(map[int64]bool)
	var closingTicketsLock sync.Mutex

	cmdCloseTicket := &commands.YAGCommand{
		CmdCategory: categoryTickets,
		Name:        "Close",
		Aliases:     []string{"end", "delete"},
		Description: "Closes the ticket",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "reason", Type: dcmd.String, Default: "none"},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)
			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

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
			common.BotSession.ChannelMessageSend(parsed.CS.ID, "Closing ticket, creating logs, downloading attachments and so on.\nThis may take a while if the ticket is big.")

			currentTicket.Ticket.ClosedAt.Time = time.Now()
			currentTicket.Ticket.ClosedAt.Valid = true

			isAdminsOnly := ticketIsAdminOnly(conf, parsed.CS)

			// create the logs, download the attachments
			err := createLogs(parsed.GS, conf, currentTicket.Ticket, isAdminsOnly)
			if err != nil {
				return nil, err
			}

			TicketLog(conf, parsed.GS.ID, parsed.Msg.Author, &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("Ticket #%d - %q closed", currentTicket.Ticket.LocalID, currentTicket.Ticket.Title),
				Description: fmt.Sprintf("Reason: %s", parsed.Args[0].Str()),
				Color:       0xf23c3c,
			})

			// if everything went well, delete the channel
			_, err = common.BotSession.ChannelDelete(currentTicket.Ticket.ChannelID)
			if err != nil {
				return nil, err
			}

			_, err = currentTicket.Ticket.UpdateG(parsed.Context(), boil.Whitelist("closed_at"))
			if err != nil {
				return nil, err
			}

			return "", nil
		},
	}

	cmdAdminsOnly := &commands.YAGCommand{
		CmdCategory: categoryTickets,
		Name:        "AdminsOnly",
		Aliases:     []string{"adminonly", "ao"},
		Description: "Toggle admins only mode for this ticket",
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {

			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)

			isAdminsOnlyCurrently := true

			modOverwrites := make([]*discordgo.PermissionOverwrite, 0)

			parsed.GS.RLock()
			for _, ow := range parsed.CS.PermissionOverwrites {
				if ow.Type == "role" && common.ContainsInt64Slice(conf.ModRoles, ow.ID) {
					if (ow.Allow & InTicketPerms) == InTicketPerms {
						// one of the mod roles has ticket perms, this is not a admin ticket currently
						isAdminsOnlyCurrently = false
					}

					modOverwrites = append(modOverwrites, ow)
				}
			}
			parsed.GS.RUnlock()

			// update existing overwrites
			for _, v := range modOverwrites {
				var err error
				if isAdminsOnlyCurrently {
					// add back the mods to this ticket
					if (v.Allow & InTicketPerms) != InTicketPerms {
						// add it back to allows, remove from denies
						newAllows := v.Allow | InTicketPerms
						newDenies := v.Deny & (InTicketPerms ^ InTicketPerms)
						err = common.BotSession.ChannelPermissionSet(parsed.CS.ID, v.ID, "role", newAllows, newDenies)
					}
				} else {
					// remove the mods from this ticket
					if (v.Allow & InTicketPerms) == InTicketPerms {
						// remove it from allows
						newAllows := v.Allow & (InTicketPerms ^ InTicketPerms)
						err = common.BotSession.ChannelPermissionSet(parsed.CS.ID, v.ID, "role", newAllows, v.Deny)
					}
				}

				if err != nil {
					logger.WithError(err).WithField("guild", parsed.GS.ID).Error("[tickets] failed to update channel overwrite")
				}
			}

			if isAdminsOnlyCurrently {
				// add the missing overwrites for the missing roles
			OUTER:
				for _, v := range conf.ModRoles {
					for _, ow := range modOverwrites {
						if ow.ID == v {
							// already handled above
							continue OUTER
						}
					}

					// need to create a new overwrite
					err := common.BotSession.ChannelPermissionSet(parsed.CS.ID, v, "role", InTicketPerms, 0)
					if err != nil {
						logger.WithError(err).WithField("guild", parsed.GS.ID).Error("[tickets] failed to create channel overwrite")
					}
				}
			}

			if isAdminsOnlyCurrently {
				return "Added back mods to the ticket", nil
			}

			return "Removed mods from this ticket", nil
		},
	}

	container := commands.CommandSystem.Root.Sub("tickets", "ticket")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")
	container.AddMidlewares(
		func(inner dcmd.RunFunc) dcmd.RunFunc {
			return func(data *dcmd.Data) (interface{}, error) {

				conf, err := models.FindTicketConfigG(data.Context(), data.GS.ID)
				if err != nil {
					if err != sql.ErrNoRows {
						return nil, err
					}

					conf = &models.TicketConfig{}
				}

				if conf.Enabled {
					go analytics.RecordActiveUnit(data.GS.ID, &Plugin{}, "cmd_used")
				}

				activeTicket, err := models.Tickets(qm.Where("channel_id = ? AND guild_id = ?", data.CS.ID, data.GS.ID)).OneG(data.Context())
				if err != nil && err != sql.ErrNoRows {
					return nil, err
				}

				// no ticket commands have any effect then
				if activeTicket == nil && !conf.Enabled {
					return "Ticket system is disabled on this server, admins can enable it in the control panel.", nil
				}

				ctx := context.WithValue(data.Context(), CtxKeyConfig, conf)

				if activeTicket != nil {
					participants, _ := models.TicketParticipants(qm.Where("ticket_guild_id = ? AND ticket_local_id = ?", activeTicket.GuildID, activeTicket.LocalID)).AllG(ctx)
					ctx = context.WithValue(ctx, CtxKeyCurrentTicket, &Ticket{
						Ticket:       activeTicket,
						Participants: participants,
					})
				}

				return inner(data.WithContext(ctx))
			}
		})

	container.AddCommand(cmdOpenTicket, cmdOpenTicket.GetTrigger())
	container.AddCommand(cmdAddParticipant, cmdAddParticipant.GetTrigger().SetMiddlewares(RequireActiveTicketMW))
	container.AddCommand(cmdRemoveParticipant, cmdRemoveParticipant.GetTrigger().SetMiddlewares(RequireActiveTicketMW))
	container.AddCommand(cmdRenameTicket, cmdRenameTicket.GetTrigger().SetMiddlewares(RequireActiveTicketMW))
	container.AddCommand(cmdCloseTicket, cmdCloseTicket.GetTrigger().SetMiddlewares(RequireActiveTicketMW))
	container.AddCommand(cmdAdminsOnly, cmdAdminsOnly.GetTrigger().SetMiddlewares(RequireActiveTicketMW))
}

func RequireActiveTicketMW(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		if data.Context().Value(CtxKeyCurrentTicket) == nil {
			return "This command can only be ran in a active ticket", nil
		}

		return inner(data)
	}
}

type CtxKey int

const (
	CtxKeyConfig        CtxKey = iota
	CtxKeyCurrentTicket CtxKey = iota
)

type Ticket struct {
	Ticket       *models.Ticket
	Participants []*models.TicketParticipant
}

func createLogs(gs *dstate.GuildState, conf *models.TicketConfig, ticket *models.Ticket, adminOnly bool) error {

	if !conf.TicketsUseTXTTranscripts && !conf.DownloadAttachments {
		return nil // nothing to do here
	}

	channelID := ticket.ChannelID

	attachments := make([][]*discordgo.MessageAttachment, 0)

	msgs := make([]*discordgo.Message, 0, 100)
	before := int64(0)

	totalAttachmentSize := 0
	for {
		m, err := common.BotSession.ChannelMessages(channelID, 100, int64(before), 0, 0)
		if err != nil {
			return err
		}

		for _, msg := range m {
			// download attachments
		OUTER:
			for _, att := range msg.Attachments {
				msg.Content += fmt.Sprintf("(attatchment: %s)", att.Filename)

				totalAttachmentSize += att.Size
				if totalAttachmentSize > 500000000 {
					// above 500MB, ignore...
					break
				}

				// group them up
				for i, ag := range attachments {
					combinedSize := 0
					for _, a := range ag {
						combinedSize += a.Size
					}

					if att.Size+combinedSize > 40000000 {
						continue
					}

					// room left in this zip file
					attachments[i] = append(ag, att)
					continue OUTER
				}

				// we didn't find a grouping
				attachments = append(attachments, []*discordgo.MessageAttachment{att})
			}
		}

		// either continue fetching more or append to messages slice
		if conf.TicketsUseTXTTranscripts {
			msgs = append(msgs, m...)
		}

		if len(msgs) > 100000 {
			break // hard limit at 100k
		}

		if len(m) == 100 {
			// More...
			before = m[len(m)-1].ID
		} else {
			break
		}
	}

	if conf.TicketsUseTXTTranscripts && gs.Channel(true, transcriptChannel(conf, adminOnly)) != nil {
		formattedTranscript := createTXTTranscript(ticket, msgs)

		channel := transcriptChannel(conf, adminOnly)
		_, err := common.BotSession.ChannelFileSendWithMessage(channel, fmt.Sprintf("transcript-%d-%s.txt", ticket.LocalID, ticket.Title), fmt.Sprintf("transcript-%d-%s.txt", ticket.LocalID, ticket.Title), formattedTranscript)
		if err != nil {
			return err
		}
	}

	// compress and send the attachments
	if conf.DownloadAttachments && gs.Channel(true, transcriptChannel(conf, adminOnly)) != nil {
		archiveAttachments(conf, ticket, attachments, adminOnly)
	}

	return nil
}

func archiveAttachments(conf *models.TicketConfig, ticket *models.Ticket, groups [][]*discordgo.MessageAttachment, adminOnly bool) {
	var buf bytes.Buffer
	for _, ag := range groups {
		if len(ag) == 1 {
			resp, err := http.Get(ag[0].URL)
			if err != nil {
				continue
			}

			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				continue
			}

			fName := fmt.Sprintf("attachments-%d-%s-%s", ticket.LocalID, ticket.Title, ag[0].Filename)
			_, err = common.BotSession.ChannelFileSendWithMessage(transcriptChannel(conf, adminOnly),
				fName, fName, resp.Body)
			continue
		}

		// zip multiple files togheter
		zw := zip.NewWriter(&buf)
		for _, v := range ag {

			resp, err := http.Get(v.URL)
			if err != nil {
				continue
			}

			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				continue
			}

			f, err := zw.Create(v.Filename)
			if err != nil {
				logger.WithError(err).Info("failed creating zip file")
				continue
			}

			_, err = io.Copy(f, resp.Body)
			if err != nil {
				continue
			}

		}

		zw.Close()
		fname := fmt.Sprintf("attachments-%d-%s.zip", ticket.LocalID, ticket.Title)
		_, err := common.BotSession.ChannelFileSendWithMessage(transcriptChannel(conf, adminOnly), fname, fname, &buf)
		buf.Reset()

		if err != nil {
			logger.WithError(err).WithField("guild", ticket.GuildID).WithField("ticket", ticket.LocalID).Error("[tickets] failed archiving batch of attachments")
		}
	}
}

const TicketTXTDateFormat = "2006 Jan 02 15:04:05"

func createTXTTranscript(ticket *models.Ticket, msgs []*discordgo.Message) *bytes.Buffer {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Transcript of ticket #%d - %s, opened by %s at %s, closed at %s.\n\n",
		ticket.LocalID, ticket.Title, ticket.AuthorUsernameDiscrim, ticket.CreatedAt.UTC().Format(TicketTXTDateFormat), ticket.ClosedAt.Time.UTC().Format(TicketTXTDateFormat)))

	// traverse reverse for correct order (they come in with new-old order, we want old-new)
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]

		// serialize mesasge content
		ts, _ := m.Timestamp.Parse()
		buf.WriteString(fmt.Sprintf("[%s] %s#%s (%d): ", ts.UTC().Format(TicketTXTDateFormat), m.Author.Username, m.Author.Discriminator, m.Author.ID))
		if m.Content != "" {
			buf.WriteString(m.Content)
			if len(m.Embeds) > 0 {
				buf.WriteString(", ")
			}
		}

		// serialize embeds
		for _, v := range m.Embeds {
			marshalled, err := json.Marshal(v)
			if err != nil {
				continue
			}

			buf.Write(marshalled)
		}

		buf.WriteRune('\n')
	}

	return &buf
}

func ticketIsAdminOnly(conf *models.TicketConfig, cs *dstate.ChannelState) bool {

	isAdminsOnlyCurrently := true

	cs.Guild.RLock()
	for _, ow := range cs.PermissionOverwrites {
		if ow.Type == "role" && common.ContainsInt64Slice(conf.ModRoles, ow.ID) {
			if (ow.Allow & InTicketPerms) == InTicketPerms {
				// one of the mod roles has ticket perms, this is not a admin ticket currently
				isAdminsOnlyCurrently = false
			}
		}
	}
	cs.Guild.RUnlock()

	return isAdminsOnlyCurrently
}

func transcriptChannel(conf *models.TicketConfig, adminOnly bool) int64 {
	if adminOnly && conf.TicketsTranscriptsChannelAdminOnly != 0 {
		return conf.TicketsTranscriptsChannelAdminOnly
	}

	return conf.TicketsTranscriptsChannel
}

func createTicketChannel(conf *models.TicketConfig, gs *dstate.GuildState, authorID int64, subject string) (int64, *discordgo.Channel, error) {
	// assemble the permission overwrites for the channel were about to create
	overwrites := []*discordgo.PermissionOverwrite{
		&discordgo.PermissionOverwrite{
			Type:  "member",
			ID:    authorID,
			Allow: InTicketPerms,
		},
		&discordgo.PermissionOverwrite{
			Type: "role",
			ID:   gs.ID,
			Deny: InTicketPerms,
		},
		&discordgo.PermissionOverwrite{
			Type:  "member",
			ID:    common.BotUser.ID,
			Allow: InTicketPerms,
		},
	}

	// add all the mod and admin roles
OUTER:
	for _, v := range conf.ModRoles {
		for _, po := range overwrites {
			if po.Type == "role" && po.ID == v {
				po.Allow |= InTicketPerms
				continue OUTER
			}
		}

		// not found in existing
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			Type:  "role",
			ID:    v,
			Allow: InTicketPerms,
		})
	}

	// add admin roles
OUTER2:
	for _, v := range conf.AdminRoles {
		for _, po := range overwrites {
			if po.Type == "role" && po.ID == v {
				po.Allow |= InTicketPerms
				continue OUTER2
			}
		}

		// not found in existing
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			Type:  "role",
			ID:    v,
			Allow: InTicketPerms,
		})
	}

	// inherit settings from category
	overwrites = applyChannelParentSettings(gs, conf.TicketsChannelCategory, overwrites)

	// generate the ID for this ticket
	id, err := common.GenLocalIncrID(gs.ID, "ticket")
	if err != nil {
		return 0, nil, err
	}

	channel, err := common.BotSession.GuildChannelCreateWithOverwrites(gs.ID, fmt.Sprintf("%d-%s", id, subject), discordgo.ChannelTypeGuildText, conf.TicketsChannelCategory, overwrites)
	if err != nil {
		return 0, nil, err
	}

	return id, channel, nil
}

func applyChannelParentSettings(gs *dstate.GuildState, parentCategoryID int64, overwrites []*discordgo.PermissionOverwrite) []*discordgo.PermissionOverwrite {
	cs := gs.ChannelCopy(true, parentCategoryID)
	if cs == nil {
		return overwrites
	}

	return applyChannelParentSettingsOverwrites(cs.PermissionOverwrites, overwrites)
}

func applyChannelParentSettingsOverwrites(parentOverwrites []*discordgo.PermissionOverwrite, newChannelOverwrites []*discordgo.PermissionOverwrite) []*discordgo.PermissionOverwrite {
OUTER:
	for _, v := range parentOverwrites {
		for _, nov := range newChannelOverwrites {
			if nov.Type == v.Type && nov.ID == v.ID {

				nov.Deny |= v.Deny
				nov.Allow |= v.Allow

				// 0 the overlapping bits on the denies
				nov.Deny ^= (nov.Deny & nov.Allow)

				continue OUTER
			}
		}

		// did not find existing overwrite, make a new one
		cop := *v
		newChannelOverwrites = append(newChannelOverwrites, &cop)
	}

	return newChannelOverwrites
}
