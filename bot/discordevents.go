package bot

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/bot/joinedguildsupdater"
	"github.com/botlabs-gg/yagpdb/v2/bot/models"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func addBotHandlers() {
	eventsystem.AddHandlerFirstLegacy(BotPlugin, HandleReady, eventsystem.EventReady)
	eventsystem.AddHandlerFirstLegacy(BotPlugin, HandleMessageCreateUpdateFirst, eventsystem.EventMessageCreate, eventsystem.EventMessageUpdate)
	eventsystem.AddHandlerSecondLegacy(BotPlugin, StateHandler, eventsystem.EventAll)

	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, EventLogger.handleEvent, eventsystem.EventAll)

	eventsystem.AddHandlerAsyncLast(BotPlugin, HandleGuildCreate, eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(BotPlugin, HandleGuildDelete, eventsystem.EventGuildDelete)

	eventsystem.AddHandlerAsyncLast(BotPlugin, HandleGuildUpdate, eventsystem.EventGuildUpdate)

	eventsystem.AddHandlerAsyncLast(BotPlugin, handleInvalidateCacheEvent,
		eventsystem.EventGuildRoleCreate,
		eventsystem.EventGuildRoleUpdate,
		eventsystem.EventGuildRoleDelete,
		eventsystem.EventChannelCreate,
		eventsystem.EventChannelUpdate,
		eventsystem.EventChannelDelete,
		eventsystem.EventGuildMemberUpdate)

	eventsystem.AddHandlerAsyncLast(BotPlugin, HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(BotPlugin, HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, HandleGuildMembersChunk, eventsystem.EventGuildMembersChunk)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, HandleReactionAdd, eventsystem.EventMessageReactionAdd)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, HandleMessageCreate, eventsystem.EventMessageCreate)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, HandleRatelimit, eventsystem.EventRateLimit)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, ReadyTracker.handleReadyOrResume, eventsystem.EventReady, eventsystem.EventResumed)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, handleResumed, eventsystem.EventResumed)
	eventsystem.AddHandlerAsyncLastLegacy(BotPlugin, HandleInteractionCreate, eventsystem.EventInteractionCreate)
}

var (
	connectedGuildsCache = common.CacheSet.RegisterSlot("bot_connected_guilds", func(_ interface{}) (interface{}, error) {
		var listedServers []int64
		err := common.RedisPool.Do(radix.Cmd(&listedServers, "SMEMBERS", "connected_guilds"))
		return listedServers, err
	}, 0)
)

var (
	// guilds redis already knows we are connected to, snapshotted on READY. discord
	// replays a GuildCreate for every guild at once after a reconnect, and for these
	// there is nothing to write, so knowing about them up front saves a redis round
	// trip per guild on the shard's only event worker.
	knownGuildsMU sync.RWMutex
	knownGuilds   = make(map[int64]struct{})
)

func isGuildKnown(guildID int64) bool {
	knownGuildsMU.RLock()
	defer knownGuildsMU.RUnlock()

	_, ok := knownGuilds[guildID]
	return ok
}

func markGuildKnown(guildIDs ...int64) {
	knownGuildsMU.Lock()
	defer knownGuildsMU.Unlock()

	for _, guildID := range guildIDs {
		knownGuilds[guildID] = struct{}{}
	}
}

func forgetGuild(guildID int64) {
	knownGuildsMU.Lock()
	defer knownGuildsMU.Unlock()

	delete(knownGuilds, guildID)
}

func HandleReady(data *eventsystem.EventData) {
	evt := data.Ready()

	commonEventsTotal.With(prometheus.Labels{"type": "Ready"}).Inc()
	RefreshStatus(ContextSession(data.Context()))

	// We pass the common.Session to the command system and that needs the user from the state
	common.BotSession.State.Lock()
	ready := discordgo.Ready{
		Version:   evt.Version,
		SessionID: evt.SessionID,
		User:      evt.User,
	}
	common.BotSession.State.Ready = ready
	common.BotSession.State.Unlock()

	var listedServers []int64
	if listedServersI, err := connectedGuildsCache.Get(0); err == nil {
		listedServers = listedServersI.([]int64)
	} else {
		logger.WithError(err).Error("Failed retrieving connected servers")
	}

	// banned servers still have to go through the full check on GuildCreate so we leave
	// them again, so they never make it into the known set
	var bannedServers []int64
	if err := common.RedisPool.Do(radix.Cmd(&bannedServers, "SMEMBERS", "banned_servers")); err != nil {
		logger.WithError(err).Error("Failed retrieving banned servers")
	}

	banned := make(map[int64]struct{}, len(bannedServers))
	for _, v := range bannedServers {
		banned[v] = struct{}{}
	}

	guilds := make([]int64, len(evt.Guilds))
	readyGuilds := make(map[int64]struct{}, len(evt.Guilds))
	for i, v := range evt.Guilds {
		guilds[i] = v.ID
		readyGuilds[v.ID] = struct{}{}
	}

	numShards := ShardManager.GetNumShards()

	stillConnected := make([]int64, 0, len(evt.Guilds))
	for _, v := range listedServers {
		shard := (v >> 22) % int64(numShards)
		if int(shard) != data.Session.ShardID {
			continue
		}

		if _, ok := readyGuilds[v]; !ok {
			logger.Info("Left server while bot was down: ", v)
			go guildRemoved(v)
			continue
		}

		if _, isBanned := banned[v]; !isBanned {
			stillConnected = append(stillConnected, v)
		}
	}

	markGuildKnown(stillConnected...)

	featureflags.BatchInitCache(guilds)
}

var guildJoinHandler = joinedguildsupdater.NewUpdater()

var metricsJoinedGuilds = promauto.NewCounter(prometheus.CounterOpts{
	Name: "yagpdb_joined_guilds",
	Help: "Guilds yagpdb newly joined",
})

var commonEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "bot_events_total",
	Help: "Common bot events",
}, []string{"type"})

func HandleGuildCreate(evt *eventsystem.EventData) (retry bool, err error) {
	g := evt.GuildCreate()
	logger.WithFields(logrus.Fields{
		"g_name": g.Name,
		"guild":  g.ID,
	}).Debug("Joined guild")

	// guilds we already knew about at READY have nothing to write here, and after a
	// reconnect that is every guild on the shard arriving at once
	if !isGuildKnown(g.ID) {
		saddRes := 0
		isBanned := false

		err = common.RedisPool.Do(radix.Pipeline(
			radix.Cmd(&saddRes, "SADD", "connected_guilds", discordgo.StrID(g.ID)),
			radix.Cmd(&isBanned, "SISMEMBER", "banned_servers", discordgo.StrID(g.ID)),
		))
		if err != nil {
			return true, errors.WithStackIf(err)
		}

		// check if this server is new
		if saddRes > 0 {
			logger.WithField("g_name", g.Name).WithField("guild", g.ID).Info("Joined new guild!")
			go eventsystem.EmitEvent(eventsystem.NewEventData(nil, eventsystem.EventNewGuild, g), eventsystem.EventNewGuild)

			metricsJoinedGuilds.Inc()
			commonEventsTotal.With(prometheus.Labels{"type": "Guild Create"}).Inc()
		}

		// check if the server is banned from using the bot
		if isBanned {
			logger.WithField("guild", g.ID).Info("Banned server tried to add bot back")
			common.BotSession.ChannelMessageSend(g.ID, "This server is banned from using this bot. Join the support server for more info.")
			err = common.BotSession.GuildLeave(g.ID)
			if err != nil {
				return CheckDiscordErrRetry(err), errors.WithStackIf(err)
			}
		} else {
			markGuildKnown(g.ID)
		}
	}

	guildJoinHandler.Incoming <- evt

	return false, nil
}

func HandleGuildDelete(evt *eventsystem.EventData) (retry bool, err error) {
	if evt.GuildDelete().Unavailable {
		// Just a guild outage
		return
	}

	logger.WithFields(logrus.Fields{
		"guild": evt.GuildDelete().ID,
	}).Info("Left guild")

	go guildRemoved(evt.GuildDelete().ID)

	return false, nil
}

func HandleGuildMemberAdd(evt *eventsystem.EventData) (retry bool, err error) {
	// ma := evt.GuildMemberAdd()
	// failedUsersCache.Delete(discordgo.StrID(ma.GuildID) + ":" + discordgo.StrID(ma.User.ID))

	guildJoinHandler.Incoming <- evt
	return false, nil
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) (retry bool, err error) {
	guildJoinHandler.Incoming <- evt
	return false, nil
}

// StateHandler updates the world state
// use AddHandlerBefore to add handler before this one, otherwise they will alwyas be after
func StateHandler(evt *eventsystem.EventData) {
	stateTracker.HandleEvent(evt.Session, evt.EvtInterface)
	// State.HandleEvent(ContextSession(evt.Context()), evt.EvtInterface)
}

func HandleGuildUpdate(evt *eventsystem.EventData) (retry bool, err error) {
	InvalidateCache(evt.GuildUpdate().Guild.ID, 0)

	g := evt.GuildUpdate().Guild

	gm := &models.JoinedGuild{
		ID:          g.ID,
		MemberCount: int64(g.MemberCount),
		OwnerID:     g.OwnerID,
		JoinedAt:    time.Now(),
		Name:        g.Name,
		Avatar:      g.Icon,
	}

	err = gm.Upsert(evt.Context(), common.PQ, true, []string{"id"}, boil.Whitelist("name", "avatar", "owner_id"), boil.Infer())
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

func handleInvalidateCacheEvent(evt *eventsystem.EventData) (bool, error) {
	if evt.GS == nil {
		return false, nil
	}

	userID := int64(0)

	if evt.Type == eventsystem.EventGuildMemberUpdate {
		userID = evt.GuildMemberUpdate().User.ID
	}

	InvalidateCache(evt.GS.ID, userID)

	return false, nil
}

func InvalidateCache(guildID, userID int64) {
	if userID != 0 {
		if err := common.RedisPool.Do(radix.Cmd(nil, "DEL", common.CacheKeyPrefix+discordgo.StrID(userID)+":guilds")); err != nil {
			logger.WithField("guild", guildID).WithField("user", userID).WithError(err).Error("failed invalidating user guilds cache")
		}
	}
	if guildID != 0 {
		if err := common.RedisPool.Do(radix.Cmd(nil, "DEL", common.CacheKeyPrefix+common.KeyGuild(guildID))); err != nil {
			logger.WithField("guild", guildID).WithField("user", userID).WithError(err).Error("failed invalidating guild cache")
		}

		if err := common.RedisPool.Do(radix.Cmd(nil, "DEL", common.CacheKeyPrefix+common.KeyGuildChannels(guildID))); err != nil {
			logger.WithField("guild", guildID).WithField("user", userID).WithError(err).Error("failed invalidating guild channels cache")
		}
	}
}

func ConcurrentEventHandler(inner eventsystem.HandlerFuncLegacy) eventsystem.HandlerFuncLegacy {
	return eventsystem.HandlerFuncLegacy(func(evt *eventsystem.EventData) {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					stack := string(debug.Stack())
					logger.WithField(logrus.ErrorKey, err).WithField("evt", evt.Type.String()).Error("Recovered from panic in (concurrent) event handler\n" + stack)
				}
			}()

			inner(evt)
		}()
	})
}

func LimitedConcurrentEventHandler(inner eventsystem.HandlerFuncLegacy, limit int64, sleepDur time.Duration) eventsystem.HandlerFuncLegacy {
	counter := new(int64)

	return eventsystem.HandlerFuncLegacy(func(evt *eventsystem.EventData) {
		go func() {
			defer func() {
				atomic.AddInt64(counter, -1)

				if err := recover(); err != nil {
					stack := string(debug.Stack())
					logger.WithField(logrus.ErrorKey, err).WithField("evt", evt.Type.String()).Error("Recovered from panic in (concurrent) event handler\n" + stack)
				}
			}()

			for {
				// spin lock
				if atomic.AddInt64(counter, 1) <= limit {
					break
				} else {
					atomic.AddInt64(counter, -1)
					time.Sleep(sleepDur)
				}
			}

			inner(evt)
		}()
	})
}

func HandleReactionAdd(evt *eventsystem.EventData) {
	ra := evt.MessageReactionAdd()
	if ra.GuildID != 0 {
		return
	}
	if ra.UserID == common.BotUser.ID {
		return
	}

	err := pubsub.Publish("dm_reaction", -1, ra)
	if err != nil {
		logger.WithError(err).Error("failed publishing dm reaction")
	}
}

func handleDmGuildInfoInteraction(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	customID := ic.MessageComponentData().CustomID
	guild_id, err := strconv.ParseInt(strings.Replace(customID, "DM_", "", 1), 10, 64)
	if err != nil {
		logger.Errorf("DM interaction received with incorrect customID: %s from user %d", customID, ic.User.ID)
	}
	gs, err := evt.Session.Guild(guild_id)
	if err != nil {
		logger.WithError(err).Errorf("Failed getting guild info for DM %s from user %d", customID, ic.User.ID)
	}
	response := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: 64},
	}
	content := ""
	if gs == nil {
		content = fmt.Sprintf("This DM was sent from server\nID: **%d**, \nI couldn't fetch more information about it.", guild_id)
	} else {
		content = fmt.Sprintf("This DM was sent from server\nID: **%d**, \nName: **%s**", guild_id, gs.Name)
	}
	response.Data.Content = content
	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &response)
	if err != nil {
		logger.WithError(err).Errorf("DM interaction response failed.")
	}
}

func respondDMInteraction(evt *eventsystem.EventData, content string) {
	ic := evt.InteractionCreate()
	err := evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		logger.WithError(err).Error("failed responding to dm interaction")
	}
}

// deferDMInteraction acknowledges the interaction straight away, for handlers
// that may outlast the time discord allows before one must be answered. The
// returned reply fills in the response once the work is done.
func deferDMInteraction(evt *eventsystem.EventData) (reply func(content string), ok bool) {
	ic := evt.InteractionCreate()
	err := evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		logger.WithError(err).Error("failed deferring dm interaction")
		return nil, false
	}

	return func(content string) {
		_, err := evt.Session.EditOriginalInteractionResponse(common.BotApplication.ID, ic.Token, &discordgo.WebhookParams{Content: content})
		if err != nil {
			logger.WithError(err).Error("failed responding to dm interaction")
		}
	}, true
}

// handleDMReportInteraction asks the recipient why they are reporting the dm
// and whether to delete it. The dm's id rides along in the modal's custom id,
// since the submit is what does the reporting.
func handleDMReportInteraction(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()

	if confDMReportChannel.GetInt() == 0 {
		respondDMInteraction(evt, "Reporting is not configured on this instance.")
		return
	}

	customID := ic.MessageComponentData().CustomID
	guildID, err := strconv.ParseInt(strings.TrimPrefix(customID, DMReportCustomIDPrefix), 10, 64)
	if err != nil || ic.Message == nil {
		logger.Errorf("DM report with malformed customID: %s from user %d", customID, ic.User.ID)
		respondDMInteraction(evt, "Something went wrong reporting this DM.")
		return
	}

	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "Report DM to YAGPDB Staff",
			CustomID: fmt.Sprintf("%s%d_%d", DMReportModalCustomIDPrefix, guildID, ic.Message.ID),
			Components: []discordgo.TopLevelComponent{
				discordgo.TextDisplay{
					Content: "### This does NOT go to the server's moderators\n" +
						"Reports go to **YAGPDB staff**, the team behind the bot. The moderators of the server this DM came from " +
						"will **not** see it, and YAGPDB staff cannot act on how a server is run, only on abuse of the bot itself.",
				},
				discordgo.Label{
					Label: "What are you reporting to YAGPDB staff?",
					Component: discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    "category",
						Placeholder: "Pick the closest match",
						MinValues:   &dmReportCategoriesPerReport,
						MaxValues:   dmReportCategoriesPerReport,
						Options:     dmReportCategoryOptions(),
						Required:    true,
					},
				},
				discordgo.Label{
					Label:       "Details for YAGPDB staff",
					Description: "At least 20 characters. Server moderators will not see this.",
					Component: discordgo.TextInput{
						CustomID:    "reason",
						Placeholder: "Describe what happened, so YAGPDB staff can act on it",
						Style:       discordgo.TextInputParagraph,
						Required:    true,
						// Long enough that a report carries an actual description.
						MinLength: 20,
						MaxLength: 1000,
					},
				},
				discordgo.Label{
					Label: "Delete this DM after reporting",
					// The combined report and delete button on older dms shares this id
					// and always deleted, so it still does unless unticked.
					Component: discordgo.Checkbox{
						CustomID: "delete",
						Default:  true,
					},
				},
			},
		},
	})
	if err != nil {
		logger.WithError(err).Error("failed opening dm report modal")
	}
}

// handleDMReportModalSubmit forwards the reported dm along with the reason to
// the configured channel, then deletes it if the recipient asked to.
func handleDMReportModalSubmit(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	data := ic.ModalSubmitData()

	reportChannel := int64(confDMReportChannel.GetInt())
	if reportChannel == 0 {
		respondDMInteraction(evt, "Reporting is not configured on this instance.")
		return
	}

	guildID, messageID, err := parseDMReportModalCustomID(data.CustomID)
	if err != nil {
		logger.Errorf("DM report modal with malformed customID: %s from user %d", data.CustomID, ic.User.ID)
		respondDMInteraction(evt, "Something went wrong reporting this DM.")
		return
	}

	// Reporting takes several requests, more than fits in the time discord
	// allows before an interaction must be answered.
	reply, ok := deferDMInteraction(evt)
	if !ok {
		return
	}

	values := dmReportModalValues(data)
	category, known := dmReportCategoryByValue(values.category)
	if !known {
		logger.Errorf("DM report with unknown category %q from user %d", values.category, ic.User.ID)
		reply("Something went wrong reporting this DM.")
		return
	}

	response := category.response
	if category.forward {
		if err := forwardDMReport(evt, reportChannel, guildID, messageID, category, values.reason); err != nil {
			logger.WithError(err).Error("failed forwarding reported dm")
			reply("Something went wrong reporting this DM.")
			return
		}
	}

	if !values.deleteDM {
		if category.forward {
			disableDMReportButton(evt.Session, ic.ChannelID, messageID)
		}
		reply(response)
		return
	}

	// Deleted only after forwarding, so a failed delete cannot lose the report.
	if err := evt.Session.ChannelMessageDelete(ic.ChannelID, messageID); err != nil {
		logger.WithError(err).Error("failed deleting reported dm")
		reply(response + "\n\nThe message could not be deleted.")
		return
	}

	reply(response + "\n\nThe message has been deleted.")
}

// forwardDMReport forwards the reported dm to the report channel, followed by
// the details, which link back to the forward so that reports arriving close
// together cannot be mixed up.
func forwardDMReport(evt *eventsystem.EventData, reportChannel, guildID, messageID int64, category dmReportCategory, reason string) error {
	ic := evt.InteractionCreate()

	server := fmt.Sprintf("`%d` (could not fetch details)", guildID)
	// The guild is only in state when it is on this node's shards.
	if gs := State.GetGuild(guildID); gs != nil {
		server = fmt.Sprintf("**%s** `%d`", gs.Name, guildID)
	} else if g, err := evt.Session.Guild(guildID); err == nil && g != nil {
		server = fmt.Sprintf("**%s** `%d`", g.Name, guildID)
	}

	// Forward the message itself rather than rebuilding it, so the report shows
	// exactly what was received, attachments and all. The context goes in its
	// own message since a forward carries no content of its own.
	forward := &discordgo.MessageSend{
		Reference: &discordgo.MessageReference{
			Type:      discordgo.MessageReferenceTypeForward,
			ChannelID: ic.ChannelID,
			MessageID: messageID,
		},
	}

	forwarded, err := common.BotSession.ChannelMessageSendComplex(reportChannel, forward)
	if err != nil {
		return err
	}

	// A sent message comes back without its guild, which the link needs.
	if forwarded.GuildID == 0 {
		forwarded.GuildID = reportChannelGuildID(evt.Session, reportChannel)
	}

	meta := &discordgo.MessageSend{
		Content: fmt.Sprintf("[Reported DM](%s) reported by **%s** `%d`, sent from server %s\n**Category:** %s\n**Details:**\n%s",
			forwarded.Link(), ic.User.String(), ic.User.ID, server, category.label, reason),
		AllowedMentions: discordgo.AllowedMentions{},
	}
	if _, err := common.BotSession.ChannelMessageSendComplex(reportChannel, meta); err != nil {
		logger.WithError(err).Error("failed sending dm report details")
	}

	return nil
}

// disableDMReportButton keeps a dm that was reported but kept from being
// reported again.
func disableDMReportButton(session *discordgo.Session, channelID, messageID int64) {
	msg, err := session.ChannelMessage(channelID, messageID)
	if err != nil {
		logger.WithError(err).Error("failed fetching reported dm")
		return
	}

	for _, component := range msg.Components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok {
			continue
		}

		for _, c := range row.Components {
			if button, ok := c.(*discordgo.Button); ok && strings.HasPrefix(button.CustomID, DMReportCustomIDPrefix) {
				button.Label = "Message Reported"
				button.Disabled = true
			}
		}
	}

	_, err = session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Components: msg.Components,
		// Flags are always sent on an edit, these are the only ones it may carry.
		Flags: msg.Flags & (discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsIsComponentsV2),
	})
	if err != nil {
		logger.WithError(err).Error("failed disabling report button on reported dm")
	}
}

var (
	reportChannelGuildMu  sync.Mutex
	reportChannelGuildIDs = make(map[int64]int64)
)

// reportChannelGuildID looks up the guild of the report channel once, as the
// channel is fixed by config and so never moves.
func reportChannelGuildID(session *discordgo.Session, channelID int64) int64 {
	reportChannelGuildMu.Lock()
	defer reportChannelGuildMu.Unlock()

	if guildID, ok := reportChannelGuildIDs[channelID]; ok {
		return guildID
	}

	channel, err := session.Channel(channelID)
	if err != nil {
		logger.WithError(err).Error("failed fetching dm report channel")
		return 0
	}

	reportChannelGuildIDs[channelID] = channel.GuildID
	return channel.GuildID
}

func parseDMReportModalCustomID(customID string) (guildID, messageID int64, err error) {
	guildPart, messagePart, ok := strings.Cut(strings.TrimPrefix(customID, DMReportModalCustomIDPrefix), "_")
	if !ok {
		return 0, 0, errors.New("missing message id")
	}

	guildID, err = strconv.ParseInt(guildPart, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	messageID, err = strconv.ParseInt(messagePart, 10, 64)
	return guildID, messageID, err
}

type dmReportValues struct {
	category string
	reason   string
	deleteDM bool
}

func dmReportModalValues(data discordgo.ModalSubmitInteractionData) (values dmReportValues) {
	for _, component := range data.Components {
		label, ok := component.(*discordgo.Label)
		if !ok {
			continue
		}

		switch input := label.Component.(type) {
		case *discordgo.SelectMenu:
			if input.CustomID == "category" && len(input.Values) > 0 {
				values.category = input.Values[0]
			}
		case *discordgo.TextInput:
			if input.CustomID == "reason" {
				values.reason = input.Value
			}
		case *discordgo.Checkbox:
			if input.CustomID == "delete" {
				values.deleteDM = input.Value
			}
		}
	}

	return values
}

// dmReportCategory is what a recipient picks when reporting a dm. Only abuse
// of the bot itself is forwarded to staff, the rest is up to the server and
// the recipient is told so instead.
type dmReportCategory struct {
	value       string
	label       string
	description string
	forward     bool
	response    string
}

const dmReportForwardedResponse = "Reported to YAGPDB staff. Thank you."

// Ordered with the categories that are not forwarded first, as those are what
// most reports are about.
var dmReportCategories = []dmReportCategory{
	{
		value:       "punishment",
		label:       "I disagree with a punishment",
		description: "You were warned, muted, kicked or banned and want to appeal",
		response:    "YAGPDB staff cannot review or undo punishments, those are decided by the server's own moderators. Contact the server's staff to appeal.",
	},
	{
		value:       "mod_abuse",
		label:       "A moderator is abusing their powers",
		description: "A server's staff are treating members unfairly",
		response:    "YAGPDB staff do not moderate servers and cannot act on how one is run. Raise it with the server's owner, or leave the server if you prefer.",
	},
	{
		value:       "unwanted",
		label:       "I don't want these DMs",
		description: "The DMs are not abusive, you just don't want them",
		response:    "These DMs are set up by the server, not by YAGPDB staff. Ask the server's staff to stop them, leave the server, or use **Delete** to remove them.",
	},
	{
		value:    "scam",
		label:    "Scam, phishing or malicious links",
		forward:  true,
		response: dmReportForwardedResponse,
	},
	{
		value:    "harassment",
		label:    "Harassment, threats or hate speech",
		forward:  true,
		response: dmReportForwardedResponse,
	},
	{
		value:    "nsfw_illegal",
		label:    "Sexual or illegal content",
		forward:  true,
		response: dmReportForwardedResponse,
	},
	{
		value:    "impersonation",
		label:    "Pretending to be Discord or YAGPDB staff",
		forward:  true,
		response: dmReportForwardedResponse,
	},
}

// A select menu needs its bounds by pointer, and a report has one category.
var dmReportCategoriesPerReport = 1

func dmReportCategoryOptions() []discordgo.SelectMenuOption {
	options := make([]discordgo.SelectMenuOption, 0, len(dmReportCategories))
	for _, category := range dmReportCategories {
		options = append(options, discordgo.SelectMenuOption{
			Value:       category.value,
			Label:       category.label,
			Description: category.description,
		})
	}

	return options
}

func dmReportCategoryByValue(value string) (dmReportCategory, bool) {
	for _, category := range dmReportCategories {
		if category.value == value {
			return category, true
		}
	}

	return dmReportCategory{}, false
}

func handleDMDeleteInteraction(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()

	if ic.Message == nil {
		respondDMInteraction(evt, "Something went wrong deleting this DM.")
		return
	}

	reply, ok := deferDMInteraction(evt)
	if !ok {
		return
	}

	if err := evt.Session.ChannelMessageDelete(ic.ChannelID, ic.Message.ID); err != nil {
		logger.WithError(err).Error("failed deleting dm")
		reply("The message could not be deleted.")
		return
	}

	reply("Deleted.")
}

func HandleInteractionCreate(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	if ic.GuildID != 0 {
		return
	}
	if ic.User == nil {
		return
	}
	if ic.User.ID == common.BotUser.ID {
		return
	}
	//handle dm message guild info interaction

	if ic.Type == discordgo.InteractionModalSubmit &&
		strings.HasPrefix(ic.ModalSubmitData().CustomID, DMReportModalCustomIDPrefix) {
		handleDMReportModalSubmit(evt)
	} else if ic.Type == discordgo.InteractionMessageComponent {
		// The server info prefix is a prefix of the others, so it goes last.
		switch customID := ic.MessageComponentData().CustomID; {
		case strings.HasPrefix(customID, DMReportCustomIDPrefix):
			handleDMReportInteraction(evt)
		case strings.HasPrefix(customID, DMDeleteCustomIDPrefix):
			handleDMDeleteInteraction(evt)
		case strings.HasPrefix(customID, DMServerInfoCustomIDPrefix):
			handleDmGuildInfoInteraction(evt)
		default:
			publishDMInteraction(ic)
		}
	} else {
		publishDMInteraction(ic)
	}
}

func publishDMInteraction(ic *discordgo.InteractionCreate) {
	err := pubsub.Publish("dm_interaction", -1, ic)
	if err != nil {
		logger.WithError(err).Error("failed publishing dm interaction")
	}
}

func HandleMessageCreate(evt *eventsystem.EventData) {
	commonEventsTotal.With(prometheus.Labels{"type": "Message Create"}).Inc()

	mc := evt.MessageCreate()
	if mc.GuildID != 0 {
		return
	}

	if mc.Author == nil || mc.Author.ID == common.BotUser.ID {
		return
	}

	err := pubsub.Publish("dm_message", -1, mc)
	if err != nil {
		logger.WithError(err).Error("failed publishing dm message")
	}
}

// HandleMessageCreateUpdateFirst transforms the message events a little to make them easier to deal with
// Message.Member.User is null from the api, so we assign it to Message.Author
func HandleMessageCreateUpdateFirst(evt *eventsystem.EventData) {
	if evt.GS == nil {
		return
	}

	if evt.Type == eventsystem.EventMessageCreate {
		msg := evt.MessageCreate()
		if !IsUserMessage(msg.Message) {
			return
		}

		if msg.Member != nil {
			msg.Member.User = msg.Author
			msg.Member.GuildID = msg.GuildID
		}

	} else {
		edit := evt.MessageUpdate()
		if !IsUserMessage(edit.Message) {
			return
		}
		edit.Member.User = edit.Author
		edit.Member.GuildID = edit.GuildID
	}
}

func HandleRatelimit(evt *eventsystem.EventData) {
	rl := evt.RateLimit()
	if !rl.TooManyRequests.Global {
		return
	}

	pubsub.PublishRatelimit(rl)
}

func handleResumed(evt *eventsystem.EventData) {
	commonEventsTotal.With(prometheus.Labels{"type": "Resumed"}).Inc()
}
