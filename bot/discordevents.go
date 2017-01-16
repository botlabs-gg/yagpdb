package bot

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"time"
)

func HandleReady(ctx context.Context, evt interface{}) {
	log.Info("Ready received!")
	ContextSession(ctx).UpdateStatus(0, "v"+common.VERSION+" :)")
}

func HandleGuildCreate(ctx context.Context, evt interface{}) {
	g := evt.(*discordgo.GuildCreate)
	log.WithFields(log.Fields{
		"g_name": g.Name,
		"guild":  g.ID,
	}).Info("Joined guild")

	n, err := ContextRedis(ctx).Cmd("SADD", "connected_guilds", g.ID).Int()
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	if n > 0 {
		log.WithField("g_name", g.Name).WithField("guild", g.ID).Info("Joined new guild!")
		go EmitEvent(ctx, EventNewGuild, evt)
	}
}

func HandleGuildDelete(ctx context.Context, evt interface{}) {
	g := evt.(*discordgo.GuildDelete)
	log.WithFields(log.Fields{
		"g_name": g.Name,
	}).Info("Left guild")

	client := ContextRedis(ctx)
	err := client.Cmd("SREM", "connected_guilds", g.ID).Err
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	go EmitGuildRemoved(client, g.ID)
}

// Makes the member is always in state when coming online
func HandlePresenceUpdate(ctx context.Context, evt interface{}) {
	p := evt.(*discordgo.PresenceUpdate)
	if p.Status == discordgo.StatusOffline {
		return
	}

	gs := State.Guild(true, p.GuildID)
	if gs == nil {
		return
	}

	m := gs.Member(true, p.User.ID)
	if m != nil && m.Member == nil {
		return
	}

	started := time.Now()
	log.WithField("guild", p.GuildID).WithField("user", p.User.ID).Info("Querying api for guildmember")
	member, err := common.BotSession.GuildMember(p.GuildID, p.User.ID)
	elapsed := time.Since(started)
	if elapsed > time.Second*3 {
		log.WithField("guild", p.GuildID).WithField("user", p.User.ID).Error("LongGMQuery: Took " + elapsed.String() + ", to query guild member! maybe ratelimits?")
	}

	if err == nil {
		gs.MemberAddUpdate(true, member)
	}
}

// StateHandler updates the world state
// use AddHandlerBefore to add handler before this one, otherwise they will alwyas be after
func StateHandler(ctx context.Context, evt interface{}) {
	State.HandleEvent(ContextSession(ctx), evt)
}

func HandleGuildUpdate(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.GuildUpdate).Guild.ID)
}

func HandleGuildRoleUpdate(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.GuildRoleUpdate).GuildID)
}

func HandleGuildRoleCreate(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.GuildRoleCreate).GuildID)
}

func HandleGuildRoleRemove(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.GuildRoleDelete).GuildID)
}

func HandleChannelCreate(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.ChannelCreate).GuildID)
}
func HandleChannelUpdate(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.ChannelUpdate).GuildID)
}
func HandleChannelDelete(ctx context.Context, evt interface{}) {
	InvalidateGuildCache(ContextRedis(ctx), evt.(*discordgo.ChannelDelete).GuildID)
}

func InvalidateGuildCache(client *redis.Client, guildID string) {
	client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuild(guildID))
	client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuildChannels(guildID))
}
