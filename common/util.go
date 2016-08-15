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

// GetRedisJson executes a get redis command and unmarshals the value into out
func GetRedisJson(client *redis.Client, key string, out interface{}) error {
	raw, err := client.Cmd("GET", key).Bytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, out)
	return err
}

// SetRedisJson marshals the vlue and runs a set redis command for key
func SetRedisJson(client *redis.Client, key string, value interface{}) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = client.Cmd("SET", key, serialized).Err
	return err
}

// GetRedisReplies is a helper func when using redis pipelines
// It retrieves n amount of replies and returns the first error it finds (but still continues to retrieve replies after that)
func GetRedisReplies(client *redis.Client, n int) ([]*redis.Reply, error) {
	var err error
	out := make([]*redis.Reply, n)
	for i := 0; i < n; i++ {
		reply := client.GetReply()
		out[i] = reply
		if reply.Err != nil && err == nil {
			err = reply.Err
		}
	}
	return out, err
}

type RedisCmd struct {
	Name string
	Args []interface{}
}

// SafeRedisCommands Will do the following commands and stop if an error occurs
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

// RefreshConnectedGuilds deletes the connected_guilds set and fill it up again
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

	client.Append("DEL", "connected_guilds")
	count := 1

	if len(guilds) > 0 {
		client.Append("SADD", args...)
		count = 2
	}

	_, err = GetRedisReplies(client, count)
	return err
}

type WrappedGuild struct {
	*discordgo.Guild
	Connected bool
}

// GetWrapped Returns a wrapped guild with connected set
func GetWrapped(in []*discordgo.Guild, client *redis.Client) ([]*WrappedGuild, error) {
	if len(in) < 1 {
		return []*WrappedGuild{}, nil
	}

	for _, g := range in {
		client.Append("SISMEMBER", "connected_guilds", g.ID)
	}

	replies, err := GetRedisReplies(client, len(in))
	if err != nil {
		return nil, err
	}

	out := make([]*WrappedGuild, len(in))
	for k, g := range in {
		isConnected, err := replies[k].Bool()
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

// DelayedMessageDelete Deletes a message after delay
func DelayedMessageDelete(session *discordgo.Session, delay time.Duration, cID, mID string) {
	time.Sleep(delay)
	err := session.ChannelMessageDelete(cID, mID)
	if err != nil {
		log.Println("Failed deleting message:", err)
	}
}

// SendTempMessage sends a message that gets deleted after duration
func SendTempMessage(session *discordgo.Session, duration time.Duration, cID, msg string) {
	m, err := BotSession.ChannelMessageSend(cID, msg)
	if err != nil {
		return
	}

	DelayedMessageDelete(session, duration, cID, m.ID)
}

// GetGuildChannels returns the guilds channels either from cache or api
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

// GetGuild returns the guild from guildid either from cache or api
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
func CreateHastebinLog(cID string) (string, error) {
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

		parsedTs, _ := time.Parse("2006-01-02T15:04:05-07:00", m.Timestamp)

		for _, attachment := range m.Attachments {
			body += fmt.Sprintf(" (Attachment: %s)", attachment.URL)
		}

		paste += fmt.Sprintf("[%s] (%19s) %20s: %s\n", parsedTs.Format("2006 "+time.Stamp), m.Author.ID, m.Author.Username, body)
	}

	resp, err := Hastebin.UploadString("Logs of #" + channel.Name + " by YAGPDB :')\n" + paste)
	if err != nil {
		return "", err
	}

	return resp.GetLink(Hastebin), nil
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
