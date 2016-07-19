package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"log"
)

func HandleReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Println("Ready received! Connected to", len(s.State.Guilds), "Guilds")
}

func HandleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Println("Joined guild", g.Name, " Connected to", len(s.State.Guilds), "Guilds")
	client, redisErr := RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer RedisPool.CarefullyPut(client, &redisErr)

	client.Append("SELECT", 0)
	client.Append("SADD", "connected_guilds", g.ID)

	replies := common.GetRedisReplies(client, 2)
	for _, reply := range replies {
		if reply.Err != nil {
			redisErr = reply.Err
			log.Println("Redis error", redisErr)
		}
	}
}

func HandleGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	log.Println("Left guild", g.Name, " Connected to", len(s.State.Guilds))

	client, redisErr := RedisPool.Get()
	if redisErr != nil {
		log.Println("Failed to get redis connection")
		return
	}
	defer RedisPool.CarefullyPut(client, &redisErr)

	client.Append("SELECT", 0)
	client.Append("SREM", "connected_guilds", g.ID)

	replies := common.GetRedisReplies(client, 2)
	for _, reply := range replies {
		if reply.Err != nil {
			redisErr = reply.Err
			log.Println("Redis error", redisErr)
		}
	}
}
