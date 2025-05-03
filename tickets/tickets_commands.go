package tickets

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/tickets/models"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

const InTicketPerms = discordgo.PermissionReadMessageHistory | discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks | discordgo.PermissionAttachFiles

var _ commands.CommandProvider = (*Plugin)(nil)

func createTicketsDisabledError(guildID int64) string {
	return fmt.Sprintf("**The tickets system is disabled for this server.** Enable it at: <%s/tickets/settings>.", web.ManageServerURL(guildID))
}

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
			{Name: "subject", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			if parsed.Context().Value(commands.CtxKeyExecutedByNestedCommandTemplate) == true {
				return nil, errors.New("cannot nest exec/execAdmin calls")
			}

			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)
			if !conf.Enabled {
				return createTicketsDisabledError(parsed.GuildData.GS.ID), nil
			}

			return openTicket(parsed.Context(), parsed.GuildData.GS, parsed.GuildData.MS, conf, parsed.Args[0].Str())
		},
	}

	cmdAddParticipant := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "AddUser",
		Description:  "Adds a user to the ticket in this channel",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "target", Type: &commands.MemberArg{}},
		},

		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Args[0].Value.(*dstate.MemberState)

			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

		OUTER:
			for _, v := range parsed.GuildData.CS.PermissionOverwrites {
				if v.Type == discordgo.PermissionOverwriteTypeMember && v.ID == target.User.ID {
					if (v.Allow & InTicketPerms) == InTicketPerms {
						return "User is already part of the ticket", nil
					}

					break OUTER
				}
			}

			err := common.BotSession.ChannelPermissionSet(currentTicket.Ticket.ChannelID, target.User.ID, discordgo.PermissionOverwriteTypeMember, InTicketPerms, 0)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Added %s to the ticket", target.User.String()), nil
		},
	}

	cmdRemoveParticipant := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "RemoveUser",
		Description:  "Removes a user from the ticket",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "target", Type: &commands.MemberArg{}},
		},

		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			target := parsed.Args[0].Value.(*dstate.MemberState)

			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)

			foundUser := false

		OUTER:
			for _, v := range parsed.GuildData.CS.PermissionOverwrites {
				if v.Type == discordgo.PermissionOverwriteTypeMember && v.ID == target.User.ID {
					if (v.Allow & InTicketPerms) == InTicketPerms {
						foundUser = true
					}

					break OUTER
				}
			}

			if !foundUser {
				return fmt.Sprintf("%s is already not (explicitly) part of this ticket", target.User.String()), nil
			}

			err := common.BotSession.ChannelPermissionDelete(currentTicket.Ticket.ChannelID, target.User.ID)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Removed %s from the ticket", target.User.String()), nil
		},
	}

	cmdRenameTicket := &commands.YAGCommand{
		CmdCategory:  categoryTickets,
		Name:         "Rename",
		Description:  "Renames the ticket",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "new-name", Type: dcmd.String},
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
			TicketLog(conf, parsed.GuildData.GS.ID, parsed.Author, &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("Ticket #%d renamed", currentTicket.Ticket.LocalID),
				Description: fmt.Sprintf("From '%s' to '%s'", oldName, newName),
				Color:       0x5394fc,
			})

			return "Renamed ticket to " + newName, nil
		},
	}

	cmdCloseTicket := &commands.YAGCommand{
		CmdCategory: categoryTickets,
		Name:        "Close",
		Aliases:     []string{"end", "delete"},
		Description: "Closes the ticket",
		Arguments: []*dcmd.ArgDef{
			{Name: "reason", Type: dcmd.String, Default: "none"},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			conf := parsed.Context().Value(CtxKeyConfig).(*models.TicketConfig)
			currentTicket := parsed.Context().Value(CtxKeyCurrentTicket).(*Ticket)
			return closeTicket(parsed.GuildData.GS, currentTicket, parsed.GuildData.CS, conf, parsed.Author, parsed.Args[0].Str(), parsed.Context())
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

			modOverwrites := make([]discordgo.PermissionOverwrite, 0)

			for _, ow := range parsed.GuildData.CS.PermissionOverwrites {
				if ow.Type == discordgo.PermissionOverwriteTypeRole && common.ContainsInt64Slice(conf.ModRoles, ow.ID) {
					if (ow.Allow & InTicketPerms) == InTicketPerms {
						// one of the mod roles has ticket perms, this is not a admin ticket currently
						isAdminsOnlyCurrently = false
					}

					modOverwrites = append(modOverwrites, ow)
				}
			}

			// update existing overwrites
			for _, v := range modOverwrites {
				var err error
				if isAdminsOnlyCurrently {
					// add back the mods to this ticket
					if (v.Allow & InTicketPerms) != InTicketPerms {
						// add it back to allows, remove from denies
						newAllows := v.Allow | InTicketPerms
						newDenies := v.Deny & (^InTicketPerms)
						err = common.BotSession.ChannelPermissionSet(parsed.ChannelID, v.ID, discordgo.PermissionOverwriteTypeRole, newAllows, newDenies)
					}
				} else {
					// remove the mods from this ticket
					if (v.Allow & InTicketPerms) == InTicketPerms {
						// remove it from allows
						newAllows := v.Allow & (^InTicketPerms)
						err = common.BotSession.ChannelPermissionSet(parsed.ChannelID, v.ID, discordgo.PermissionOverwriteTypeRole, newAllows, v.Deny)
					}
				}

				if err != nil {
					logger.WithError(err).WithField("guild", parsed.GuildData.GS.ID).Error("[tickets] failed to update channel overwrite")
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
					err := common.BotSession.ChannelPermissionSet(parsed.ChannelID, v, discordgo.PermissionOverwriteTypeRole, InTicketPerms, 0)
					if err != nil {
						logger.WithError(err).WithField("guild", parsed.GuildData.GS.ID).Error("[tickets] failed to create channel overwrite")
					}
				}
			}

			if isAdminsOnlyCurrently {
				return "Added back mods to the ticket", nil
			}

			return "Removed mods from this ticket", nil
		},
	}

	const emojiRegex = `\A\s*((<a?:[\w~]{2,32}:\d{17,19}>)|[\x{1f1e6}-\x{1f1ff}]{2}|\p{So}\x{fe0f}?[\x{1f3fb}-\x{1f3ff}]?(\x{200D}\p{So}\x{fe0f}?[\x{1f3fb}-\x{1f3ff}]?)*|[#\d*]\x{FE0F}?\x{20E3})`

	cmdMenuCreate := &commands.YAGCommand{
		CmdCategory:         categoryTickets,
		Name:                "MenuCreate",
		Aliases:             []string{"mc"},
		Description:         "Creates a menu with buttons to open tickets.",
		LongDescription:     "Creates and sends a message with buttons allowing users to open tickets, optionally with predefined reasons.\n\nInstead of creating a new message, attach it to another message the bot has sent with `-message bot-message-id-here`. This __must__ be a message the bot has sent.\nCreate buttons with up to 9 predefined reasons with `-button-1 \"Reason for button 1\"`, `-button-2 \"Reason for button 2\"`, etc.\nIf using predefined reason buttons, you may optionally disable the custom reason button with `-disable-custom`.",
		RequireDiscordPerms: []int64{discordgo.PermissionManageGuild},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "message", Help: "ID to attach menu to", Type: dcmd.BigInt},
			{Name: "disable-custom", Help: "Disable Cutsom Reason button", Default: false},
			{Name: "button-1", Help: "Predefined reason for button 1", Type: dcmd.String},
			{Name: "button-2", Help: "Predefined reason for button 2", Type: dcmd.String},
			{Name: "button-3", Help: "Predefined reason for button 3", Type: dcmd.String},
			{Name: "button-4", Help: "Predefined reason for button 4", Type: dcmd.String},
			{Name: "button-5", Help: "Predefined reason for button 5", Type: dcmd.String},
			{Name: "button-6", Help: "Predefined reason for button 6", Type: dcmd.String},
			{Name: "button-7", Help: "Predefined reason for button 7", Type: dcmd.String},
			{Name: "button-8", Help: "Predefined reason for button 8", Type: dcmd.String},
			{Name: "button-9", Help: "Predefined reason for button 9", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			var components []discordgo.InteractiveComponent
			var usedReasons []string
			for i := 1; i < 10; i++ {
				arg := parsed.Switches["button-"+strconv.Itoa(i)]
				if arg.Value == nil {
					continue
				}

				reason := arg.Str()
				emoji := &discordgo.ComponentEmoji{}
				reason = regexp.MustCompile(emojiRegex).ReplaceAllStringFunc(reason, func(match string) string {
					// <:ye:733037741532643428>
					customEmojiParts := strings.Split(match, ":")
					// []string{"<", "ye", "733037741532643428>"}
					if len(customEmojiParts) == 1 {
						// not a custom emoji, should be unicode
						emoji.Name = match
						return ""
					}
					lastPart := customEmojiParts[len(customEmojiParts)-1]
					// "733037741532643428>"
					customEmojiID := lastPart[:len(lastPart)-1] // trim off ">"
					var err error
					emoji.ID, err = strconv.ParseInt(customEmojiID, 10, 64)
					if err != nil {
						// this might not be an emoji after all, leave it untouched
						return match
					}
					return ""
				})
				reason = strings.TrimSpace(reason)

				if common.ContainsStringSlice(usedReasons, reason) {
					return "You may not use the exact same reason on multiple buttons", nil
				}
				label := reason
				if len(label) > 80 {
					label = label[:80]
				}
				button := discordgo.Button{
					Label:    label,
					CustomID: "tickets-open-" + reason,
				}

				if emoji.ID != 0 || emoji.Name != "" {
					button.Emoji = emoji
				}
				components = append(components, button)
				usedReasons = append(usedReasons, reason)
			}

			if len(components) == 0 || !parsed.Switches["disable-custom"].Bool() {
				label := "Create a Ticket"
				if len(components) > 0 {
					label = "Custom Reason"
				}
				customButton := discordgo.Button{
					Label:    label,
					CustomID: "tickets-open-",
					Style:    discordgo.SecondaryButton,
				}
				components = append([]discordgo.InteractiveComponent{customButton}, components...)
			}

			var actionsRows []discordgo.TopLevelComponent
			if len(components) > 5 {
				actionsRows = append(actionsRows, discordgo.ActionsRow{Components: components[:5]})
				components = components[5:]
			}
			actionsRows = append(actionsRows, discordgo.ActionsRow{Components: components})

			var err error
			if parsed.Switches["message"].Int64() != 0 {
				message, err := common.BotSession.ChannelMessage(parsed.ChannelID, parsed.Switches["message"].Int64())
				if err != nil {
					return nil, err
				}
				if message.Author.ID != common.BotUser.ID {
					return "You must select a message that YAGPDB has sent.", nil
				}

				_, err = common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
					Content:    &message.Content,
					Components: actionsRows,
					Embeds:     message.Embeds,
					ID:         message.ID,
					Channel:    parsed.ChannelID,
				})
			} else {
				_, err = common.BotSession.ChannelMessageSendComplex(parsed.ChannelID, &discordgo.MessageSend{
					Content:    "Click below to create a new ticket.",
					Components: actionsRows,
				})
			}
			return nil, err
		},
	}

	container, _ := commands.CommandSystem.Root.Sub("tickets", "ticket")
	container.Description = "Command to manage the ticket system"
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")
	container.AddMidlewares(
		func(inner dcmd.RunFunc) dcmd.RunFunc {
			return func(data *dcmd.Data) (interface{}, error) {

				conf, err := models.FindTicketConfigG(data.Context(), data.GuildData.GS.ID)
				if err != nil {
					if err != sql.ErrNoRows {
						return nil, err
					}

					conf = &models.TicketConfig{}
				}

				if conf.Enabled {
					go analytics.RecordActiveUnit(data.GuildData.GS.ID, &Plugin{}, "cmd_used")
				}

				activeTicket, err := models.Tickets(qm.Where("channel_id = ? AND guild_id = ?", data.GuildData.CS.ID, data.GuildData.GS.ID)).OneG(data.Context())
				if err != nil && err != sql.ErrNoRows {
					return nil, err
				}

				// no ticket commands have any effect then
				if activeTicket == nil && !conf.Enabled {
					return createTicketsDisabledError(data.GuildData.GS.ID), nil
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
	container.AddCommand(cmdMenuCreate, cmdMenuCreate.GetTrigger().SetMiddlewares(ProhibitActiveTicketMW))

	commands.RegisterSlashCommandsContainer(container, false, TicketCommandsRolesRunFuncfunc)
}

func TicketCommandsRolesRunFuncfunc(gs *dstate.GuildSet) ([]int64, error) {
	conf, err := models.FindTicketConfigG(context.Background(), gs.ID)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}

		conf = &models.TicketConfig{}
	}

	if !conf.Enabled {
		return nil, nil
	}

	// use the everyone role to signify that everyone can use the commands
	return []int64{gs.ID}, nil
}

func RequireActiveTicketMW(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		if data.Context().Value(CtxKeyCurrentTicket) == nil {
			return "This command can only be run in a active ticket", nil
		}

		return inner(data)
	}
}

func ProhibitActiveTicketMW(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		if data.Context().Value(CtxKeyCurrentTicket) != nil {
			return "This command cannot be run in a active ticket", nil
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

func createLogs(gs *dstate.GuildSet, conf *models.TicketConfig, ticket *models.Ticket, adminOnly bool) error {

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
			for _, att := range msg.GetMessageAttachments() {
				msg.Content += fmt.Sprintf("(attachment: %s)", att.Filename)

				totalAttachmentSize += att.Size
				if totalAttachmentSize > 100000000 {
					// above 100MB, ignore...
					break
				}

				// group them up
				for i, ag := range attachments {
					combinedSize := 0
					for _, a := range ag {
						combinedSize += a.Size
					}

					if att.Size+combinedSize > 8000000 {
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

	if conf.TicketsUseTXTTranscripts && gs.GetChannel(transcriptChannel(conf, adminOnly)) != nil {
		formattedTranscript := createTXTTranscript(ticket, msgs)

		channel := transcriptChannel(conf, adminOnly)
		_, err := common.BotSession.ChannelFileSendWithMessage(channel, fmt.Sprintf("transcript-%d-%s.txt", ticket.LocalID, ticket.Title), fmt.Sprintf("transcript-%d-%s.txt", ticket.LocalID, ticket.Title), formattedTranscript)
		if err != nil {
			return err
		}
	}

	// compress and send the attachments
	if conf.DownloadAttachments && gs.GetChannel(transcriptChannel(conf, adminOnly)) != nil {
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
			_, _ = common.BotSession.ChannelFileSendWithMessage(transcriptChannel(conf, adminOnly),
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

		// serialize message content
		ts, _ := m.Timestamp.Parse()
		buf.WriteString(fmt.Sprintf("[%s] %s (%d): ", ts.UTC().Format(TicketTXTDateFormat), m.Author.String(), m.Author.ID))
		contents := m.GetMessageContents()
		if len(contents) > 0 {
			for _, c := range contents {
				if c != "" {
					buf.WriteString(c)
				}
			}
			if len(m.GetMessageEmbeds()) > 0 {
				buf.WriteString(", ")
			}
		}

		// serialize embeds
		for _, v := range m.GetMessageEmbeds() {
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

	for _, ow := range cs.PermissionOverwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeRole && common.ContainsInt64Slice(conf.ModRoles, ow.ID) {
			if (ow.Allow & InTicketPerms) == InTicketPerms {
				// one of the mod roles has ticket perms, this is not a admin ticket currently
				isAdminsOnlyCurrently = false
			}
		}
	}

	return isAdminsOnlyCurrently
}

func transcriptChannel(conf *models.TicketConfig, adminOnly bool) int64 {
	if adminOnly && conf.TicketsTranscriptsChannelAdminOnly != 0 {
		return conf.TicketsTranscriptsChannelAdminOnly
	}

	return conf.TicketsTranscriptsChannel
}

func createTicketChannel(conf *models.TicketConfig, gs *dstate.GuildSet, authorID int64, subject string) (int64, *discordgo.Channel, error) {
	// assemble the permission overwrites for the channel were about to create
	overwrites := []*discordgo.PermissionOverwrite{
		{
			Type:  discordgo.PermissionOverwriteTypeMember,
			ID:    authorID,
			Allow: InTicketPerms,
		},
		{
			Type: discordgo.PermissionOverwriteTypeRole,
			ID:   gs.ID,
			Deny: InTicketPerms,
		},
		{
			Type:  discordgo.PermissionOverwriteTypeMember,
			ID:    common.BotUser.ID,
			Allow: InTicketPerms | discordgo.PermissionManageChannels,
		},
	}

	// add all the mod and admin roles
OUTER:
	for _, v := range conf.ModRoles {
		for _, po := range overwrites {
			if po.Type == discordgo.PermissionOverwriteTypeRole && po.ID == v {
				po.Allow |= InTicketPerms
				continue OUTER
			}
		}

		// not found in existing
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			Type:  discordgo.PermissionOverwriteTypeRole,
			ID:    v,
			Allow: InTicketPerms,
		})
	}

	// add admin roles
OUTER2:
	for _, v := range conf.AdminRoles {
		for _, po := range overwrites {
			if po.Type == discordgo.PermissionOverwriteTypeRole && po.ID == v {
				po.Allow |= InTicketPerms
				continue OUTER2
			}
		}

		// not found in existing
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			Type:  discordgo.PermissionOverwriteTypeRole,
			ID:    v,
			Allow: InTicketPerms,
		})
	}

	// inherit settings from category
	// TODO: disabled because of a issue with discord recently pushed change that disallows bots from creating channels with permissions they don't have
	// TODO: automatically filter those out
	// overwrites = applyChannelParentSettings(gs, conf.TicketsChannelCategory, overwrites)

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

func applyChannelParentSettings(gs *dstate.GuildSet, parentCategoryID int64, overwrites []*discordgo.PermissionOverwrite) []*discordgo.PermissionOverwrite {
	cs := gs.GetChannel(parentCategoryID)
	if cs == nil {
		return overwrites
	}

	channel_overwrites := make([]*discordgo.PermissionOverwrite, len(cs.PermissionOverwrites))
	for i := 0; i < len(overwrites); i++ {
		channel_overwrites[i] = &cs.PermissionOverwrites[i]
	}

	return applyChannelParentSettingsOverwrites(channel_overwrites, overwrites)
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
