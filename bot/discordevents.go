package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"log"
)

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Println("Ready received! Connected to", len(s.State.Guilds), "Guilds")
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, client *redis.Client) {
	log.Println("Joined guild", g.Name, " Connected to", len(s.State.Guilds), "Guilds")

	err := client.Cmd("SADD", "connected_guilds", g.ID).Err
	if err != nil {
		log.Println("Redis error", err)
	}
}

func HandleGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete, client *redis.Client) {
	log.Println("Left guild", g.Name, " Connected to", len(s.State.Guilds), "Guilds")

	err := client.Cmd("SREM", "connected_guilds", g.ID).Err

	if err != nil {
		log.Println("Redis error", err)
	}
}
