package bot

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
)

func HandleReady(evt *eventsystem.EventData) {
	log.Info("Ready received!")
	ContextSession(evt.Context()).UpdateStatus(0, "v"+common.VERSION+" :)")

	// We pass the common.Session to the command system and that needs the user from the state
	common.BotSession.State.Lock()
	ready := discordgo.Ready{
		Version:   evt.Ready.Version,
		SessionID: evt.Ready.SessionID,
		User:      evt.Ready.User,
	}
	common.BotSession.State.Ready = ready
	common.BotSession.State.Unlock()
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	g := evt.GuildCreate
	log.WithFields(log.Fields{
		"g_name": g.Name,
		"guild":  g.ID,
	}).Info("Joined guild")

	n, err := ContextRedis(evt.Context()).Cmd("SADD", "connected_guilds", g.ID).Int()
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	if n > 0 {
		log.WithField("g_name", g.Name).WithField("guild", g.ID).Info("Joined new guild!")
		go eventsystem.EmitEvent(&eventsystem.EventData{
			EventDataContainer: &eventsystem.EventDataContainer{
				GuildCreate: g,
			},
			Type: eventsystem.EventNewGuild,
		}, eventsystem.EventNewGuild)
	}
}

func HandleGuildDelete(evt *eventsystem.EventData) {
	if evt.GuildDelete.Unavailable {
		// Just a guild outage
		return
	}

	log.WithFields(log.Fields{
		"g_name": evt.GuildDelete.Name,
	}).Info("Left guild")

	client := ContextRedis(evt.Context())
	err := client.Cmd("SREM", "connected_guilds", evt.GuildDelete.ID).Err
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	go EmitGuildRemoved(client, evt.GuildDelete.ID)
}

// Makes sure the member is always in state when coming online
func HandlePresenceUpdate(evt *eventsystem.EventData) {
	p := evt.PresenceUpdate
	if p.Status == discordgo.StatusOffline {
		return
	}

	gs := State.Guild(true, p.GuildID)
	if gs == nil {
		return
	}

	m := gs.Member(true, p.User.ID)
	if m != nil && m.Member != nil {
		return
	}

	GetMember(p.GuildID, p.User.ID)
}

// StateHandler updates the world state
// use AddHandlerBefore to add handler before this one, otherwise they will alwyas be after
func StateHandler(evt *eventsystem.EventData) {
	State.HandleEvent(ContextSession(evt.Context()), evt.EvtInterface)
}

func HandleGuildUpdate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.GuildUpdate.Guild.ID, "")
}

func HandleGuildRoleUpdate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.GuildRoleUpdate.GuildID, "")
}

func HandleGuildRoleCreate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.GuildRoleCreate.GuildID, "")
}

func HandleGuildRoleRemove(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.GuildRoleDelete.GuildID, "")
}

func HandleChannelCreate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.ChannelCreate.GuildID, "")
}
func HandleChannelUpdate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.ChannelUpdate.GuildID, "")
}
func HandleChannelDelete(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), evt.ChannelDelete.GuildID, "")
}

func HandleGuildMemberUpdate(evt *eventsystem.EventData) {
	InvalidateCache(ContextRedis(evt.Context()), "", evt.GuildMemberUpdate.User.ID)
}

func InvalidateCache(client *redis.Client, guildID, userID string) {
	if userID != "" {
		client.Cmd("DEL", common.CacheKeyPrefix+userID+":guilds")
	}
	if guildID != "" {
		client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuild(guildID))
		client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuildChannels(guildID))
	}
}

func ConcurrentEventHandler(inner eventsystem.Handler) eventsystem.Handler {
	return func(evt *eventsystem.EventData) {
		go inner(evt)
	}
}
