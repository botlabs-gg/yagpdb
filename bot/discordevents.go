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

	n, err := client.Cmd("SADD", "connected_guilds", g.ID).Int()
	if err != nil {
		log.WithError(err).Error("Redis error")
	}

	if n > 0 {
		log.WithField("g_name", g.Name).Info("Joined new guild!")
	}

	// Reset this stat
	err = client.Cmd("SET", "guild_stats_num_members:"+g.ID, 0).Err
	if err != nil {
		log.WithError(err).Error("Failed resetting guild members stat")
	}

	LoadGuildMembersQueue <- g.ID

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

func HandleGuildMembersChunk(s *discordgo.Session, g *discordgo.GuildMembersChunk, client *redis.Client) {
	log.WithFields(log.Fields{
		"num_members": len(g.Members),
		"guild":       g.GuildID,
	}).Info("Received guild members")

	// Load all members in memory, this might cause issues in the future, who knows /shrug
	for _, v := range g.Members {
		v.GuildID = g.GuildID
		err := common.BotSession.State.MemberAdd(v)
		if err != nil {
			log.WithError(err).Error("Failed adding member to state")
		}
	}

	err := client.Cmd("INCRBY", "guild_stats_num_members:"+g.GuildID, len(g.Members)).Err
	if err != nil {
		log.WithError(err).Error("Failed increasing guild members stat")
	}
}
