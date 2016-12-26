package bot

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Info("Ready received!")
	s.UpdateStatus(0, "v"+common.VERSION+" :)")
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	log.WithFields(log.Fields{
		"g_name": g.Name,
		"guild":  g.ID,
	}).Info("Joined guild")

	n, err := client.Cmd("SADD", "connected_guilds", g.ID).Int()
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	if n > 0 {
		log.WithField("g_name", g.Name).WithField("guild", g.ID).Info("Joined new guild!")
	}
}

func HandleGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete, client *redis.Client) {

	log.WithFields(log.Fields{
		"g_name": g.Name,
	}).Info("Left guild")

	err := client.Cmd("SREM", "connected_guilds", g.ID).Err
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	EmitGuildRemoved(client, g.ID)
}

func HandleGuildUpdate(s *discordgo.Session, evt *discordgo.GuildUpdate, client *redis.Client) {
	InvalidateGuildCache(client, evt.Guild.ID)
}

func HandleGuildRoleUpdate(s *discordgo.Session, evt *discordgo.GuildRoleUpdate, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}

func HandleGuildRoleCreate(s *discordgo.Session, evt *discordgo.GuildRoleCreate, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}

func HandleGuildRoleRemove(s *discordgo.Session, evt *discordgo.GuildRoleDelete, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}

func HandleChannelCreate(s *discordgo.Session, evt *discordgo.ChannelCreate, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}
func HandleChannelUpdate(s *discordgo.Session, evt *discordgo.ChannelUpdate, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}
func HandleChannelDelete(s *discordgo.Session, evt *discordgo.ChannelDelete, client *redis.Client) {
	InvalidateGuildCache(client, evt.GuildID)
}

func InvalidateGuildCache(client *redis.Client, guildID string) {
	client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuild(guildID))
	client.Cmd("DEL", common.CacheKeyPrefix+common.KeyGuildChannels(guildID))
}
