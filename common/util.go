package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"log"
	"time"
)

func GetRedisJson(client *redis.Client, key string, out interface{}) error {
	raw, err := client.Cmd("GET", key).Bytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, out)
	return err
}

func SetRedisJson(client *redis.Client, key string, value interface{}) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = client.Cmd("SET", key, serialized).Err
	return err
}

func GetRedisReplies(client *redis.Client, n int) []*redis.Reply {
	out := make([]*redis.Reply, n)
	for i := 0; i < n; i++ {
		out[i] = client.GetReply()
	}
	return out
}

type RedisCmd struct {
	Name string
	Args []interface{}
}

// Will do the following commands and stop if an error occurs
func SafeRedisCommands(client *redis.Client, cmds []*RedisCmd) ([]*redis.Reply, error) {
	out := make([]*redis.Reply, 0)
	for _, cmd := range cmds {
		reply := client.Cmd(cmd.Name, cmd.Args...)
		out = append(out, reply)
		if reply.Err != nil {
			return out, reply.Err
		}
	}
	return out, nil
}

// Will delete the connected_guilds set and fill it up again
// This is incase servers are removed/bot left servers while it was offline
func RefreshConnectedGuilds(session *discordgo.Session, client *redis.Client) error {
	guilds, err := session.UserGuilds()
	if err != nil {
		return err
	}

	args := make([]interface{}, len(guilds)+1)
	for k, v := range guilds {
		args[k+1] = v.ID
	}
	args[0] = "connected_guilds"

	client.Append("SELECT", 0)
	client.Append("DEL", "connected_guilds")
	count := 2

	if len(guilds) > 0 {
		client.Append("SADD", args...)
		count = 3
	}

	replies := GetRedisReplies(client, count)
	for _, v := range replies {
		if v.Err != nil {
			return v.Err
		}
	}
	return nil
}

type WrappedGuild struct {
	*discordgo.Guild
	Connected bool
}

// Returns a wrapped guild with connected set
func GetWrapped(in []*discordgo.Guild, client *redis.Client) ([]*WrappedGuild, error) {
	client.Append("SELECT", 0)
	for _, g := range in {
		client.Append("SISMEMBER", "connected_guilds", g.ID)
	}

	replies := GetRedisReplies(client, len(in)+1)

	if replies[0].Err != nil {
		return nil, replies[0].Err
	}

	out := make([]*WrappedGuild, len(in))
	for k, g := range in {
		isConnected, err := replies[k+1].Bool()
		if err != nil {
			return nil, err
		}
		out[k] = &WrappedGuild{
			Guild:     g,
			Connected: isConnected,
		}
	}
	return out, nil
}

func DelayedMessageDelete(session *discordgo.Session, delay time.Duration, cID, mID string) {
	time.Sleep(delay)
	err := session.ChannelMessageDelete(cID, mID)
	if err != nil {
		log.Println("Failed deleting message:", err)
	}
}

// Helper methods that also checks the cache
func GetGuildChannels(client *redis.Client, guildID string) (channels []*discordgo.Channel, err error) {
	// Check cache first
	err = GetCacheDataJson(client, "channels:"+guildID, &channels)
	if err != nil {
		channels, err = BotSession.GuildChannels(guildID)
		if err == nil {
			SetCacheDataJsonSimple(client, "channels:"+guildID, channels)
		}
	}

	return
}

func GetGuild(client *redis.Client, guildID string) (guild *discordgo.Guild, err error) {
	// Check cache first
	err = GetCacheDataJson(client, "guild:"+guildID, &guild)
	if err != nil {
		guild, err = BotSession.Guild(guildID)
		if err == nil {
			SetCacheDataJsonSimple(client, "guild:"+guildID, guild)
		}
	}

	return
}

// Creates a pastebin log form the last 100 messages in a channel
// Returns the id of the paste
func CreatePastebinLog(cID string) (string, error) {
	channel, err := BotSession.State.Channel(cID)
	if err != nil {
		return "", err
	}

	msgs, err := GetMessages(cID, 100)
	if err != nil {
		return "", err
	}

	if len(msgs) < 1 {
		return "", errors.New("No messages in channel")
	}

	paste := ""

	for _, m := range msgs {
		body := m.ContentWithMentionsReplaced()

		for _, attachment := range m.Attachments {
			body += fmt.Sprintf(" (Attachment: %s)", attachment.URL)
		}

		paste += fmt.Sprintf("[%s] #%s, %s (%s): %s\n", m.Timestamp, channel.Name, m.Author.Username, m.Author.ID, body)
	}

	id, err := Pastebin.Put(paste, "#"+channel.Name+" Logs")
	return id, err
}

// This was bad on large servers...

// func GetGuildMembers(client *redis.Client, guildID string) (members []*discordgo.Member, err error) {
// 	err = GetCacheDataJson(client, "guild_members:"+guildID, &members)
// 	if err == nil {
// 		return
// 	}

// 	members, err = dutil.GetAllGuildMembers(BotSession, guildID)
// 	if err != nil {
// 		return
// 	}

// 	SetCacheDataJsonSimple(client, "guild_members:"+guildID, members)
// 	return
// }
