package bot

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
)

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.WithField("num_guilds", len(s.State.Guilds)).Info("Ready received!")
	s.UpdateStatus(0, "v"+common.VERSION+" :)")
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	log.WithFields(log.Fields{
		"num_guilds": len(s.State.Guilds),
		"g_name":     g.Name,
	}).Info("Joined guild")

	err := client.Cmd("SADD", "connected_guilds", g.ID).Err
	if err != nil {
		log.WithError(err).Error("Redis error")
	}
}

func HandleGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete, client *redis.Client) {
	log.WithFields(log.Fields{
		"num_guilds": len(s.State.Guilds),
		"g_name":     g.Name,
	}).Info("Left guild")

	err := client.Cmd("SREM", "connected_guilds", g.ID).Err
	if err != nil {
		log.WithError(err).Error("Redis error")
	}
}
