package bot

import (
	"emperror.dev/errors"
	"runtime/debug"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/bot/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

func addBotHandlers() {
	eventsystem.AddHandlerFirstLegacy(BotPlugin, HandleReady, eventsystem.EventReady)
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
}

func HandleReady(data *eventsystem.EventData) {
	evt := data.Ready()

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
	err := common.RedisPool.Do(retryableredis.Cmd(&listedServers, "SMEMBERS", "connected_guilds"))
	if err != nil {
		logger.WithError(err).Error("Failed retrieving connected servers")
	}

	numShards := ShardManager.GetNumShards()

OUTER:
	for _, v := range listedServers {
		shard := (v >> 22) % int64(numShards)
		if int(shard) != data.Session.ShardID {
			continue
		}

		for _, readyGuild := range evt.Guilds {
			if readyGuild.ID == v {
				continue OUTER
			}
		}

		logger.Info("Left server while bot was down: ", v)
		go guildRemoved(v)
	}
}

func HandleGuildCreate(evt *eventsystem.EventData) (retry bool, err error) {
	g := evt.GuildCreate()
	logger.WithFields(logrus.Fields{
		"g_name": g.Name,
		"guild":  g.ID,
	}).Debug("Joined guild")

	var n int
	err = common.RedisPool.Do(retryableredis.Cmd(&n, "SADD", "connected_guilds", discordgo.StrID(g.ID)))
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	// check if this server is new
	if n > 0 {
		logger.WithField("g_name", g.Name).WithField("guild", g.ID).Info("Joined new guild!")
		go eventsystem.EmitEvent(eventsystem.NewEventData(nil, eventsystem.EventNewGuild, g), eventsystem.EventNewGuild)

		if common.Statsd != nil {
			common.Statsd.Incr("yagpdb.joined_guilds", nil, 1)
		}
	}

	// check if the server is banned from using the bot
	var banned bool
	common.RedisPool.Do(retryableredis.Cmd(&banned, "SISMEMBER", "banned_servers", discordgo.StrID(g.ID)))
	if banned {
		logger.WithField("guild", g.ID).Info("Banned server tried to add bot back")
		common.BotSession.ChannelMessageSend(g.ID, "This server is banned from using this bot. Join the support server for more info.")
		err = common.BotSession.GuildLeave(g.ID)
		if err != nil {
			return CheckDiscordErrRetry(err), errors.WithStackIf(err)
		}
	}

	gm := &models.JoinedGuild{
		ID:          g.ID,
		MemberCount: int64(g.MemberCount),
		OwnerID:     g.OwnerID,
		JoinedAt:    time.Now(),
		Name:        g.Name,
		Avatar:      g.Icon,
	}

	err = gm.Upsert(evt.Context(), common.PQ, true, []string{"id"}, boil.Whitelist("member_count", "name", "avatar", "owner_id", "left_at"), boil.Infer())
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

func HandleGuildDelete(evt *eventsystem.EventData) (retry bool, err error) {
	if evt.GuildDelete().Unavailable {
		// Just a guild outage
		return
	}

	logger.WithFields(logrus.Fields{
		"g_name": evt.GuildDelete().Name,
	}).Info("Left guild")

	go guildRemoved(evt.GuildDelete().ID)

	return false, nil
}

func HandleGuildMemberAdd(evt *eventsystem.EventData) (retry bool, err error) {
	ma := evt.GuildMemberAdd()

	failedUsersCache.Delete(discordgo.StrID(ma.GuildID) + ":" + discordgo.StrID(ma.User.ID))

	_, err = common.PQ.Exec("UPDATE joined_guilds SET member_count = member_count + 1 WHERE id = $1", ma.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

func HandleGuildMemberRemove(evt *eventsystem.EventData) (retry bool, err error) {
	mr := evt.GuildMemberRemove()

	_, err = common.PQ.Exec("UPDATE joined_guilds SET member_count = member_count - 1 WHERE id = $1", mr.GuildID)
	if err != nil {
		return true, errors.WithStackIf(err)
	}

	return false, nil
}

// StateHandler updates the world state
// use AddHandlerBefore to add handler before this one, otherwise they will alwyas be after
func StateHandler(evt *eventsystem.EventData) {
	State.HandleEvent(ContextSession(evt.Context()), evt.EvtInterface)
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
		if err := common.RedisPool.Do(retryableredis.Cmd(nil, "DEL", common.CacheKeyPrefix+discordgo.StrID(userID)+":guilds")); err != nil {
			logger.WithField("guild", guildID).WithField("user", userID).WithError(err).Error("failed invalidating user guilds cache")
		}
	}
	if guildID != 0 {
		if err := common.RedisPool.Do(retryableredis.Cmd(nil, "DEL", common.CacheKeyPrefix+common.KeyGuild(guildID))); err != nil {
			logger.WithField("guild", guildID).WithField("user", userID).WithError(err).Error("failed invalidating guild cache")
		}

		if err := common.RedisPool.Do(retryableredis.Cmd(nil, "DEL", common.CacheKeyPrefix+common.KeyGuildChannels(guildID))); err != nil {
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

func HandleMessageCreate(evt *eventsystem.EventData) {
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

func HandleRatelimit(evt *eventsystem.EventData) {
	rl := evt.RateLimit()
	if !rl.TooManyRequests.Global {
		return
	}

	logger.Printf("Got 429: %s, %d", rl.Bucket, rl.RetryAfter)

	reset := time.Now().Add(rl.RetryAfter * time.Millisecond)
	err := pubsub.Publish("global_ratelimit", -1, &GlobalRatelimitTriggeredEventData{
		Bucket: rl.Bucket,
		Reset:  reset,
	})
	if err != nil {
		logger.WithError(err).Error("failed publishing global ratelimit")
	}
}
