package common

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"math/rand"
	"text/template"
	"time"
)

func KeyGuild(guildID string) string         { return "guild:" + guildID }
func KeyGuildChannels(guildID string) string { return "channels:" + guildID }

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
	*discordgo.UserGuild
	Connected bool
}

// GetWrapped Returns a wrapped guild with connected set
func GetWrapped(in []*discordgo.UserGuild, client *redis.Client) ([]*WrappedGuild, error) {
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
			UserGuild: g,
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
		log.WithError(err).Error("Failed deleing message")
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
	err = GetCacheDataJson(client, KeyGuildChannels(guildID), &channels)
	if err != nil {
		channels, err = BotSession.GuildChannels(guildID)
		if err == nil {
			SetCacheDataJsonSimple(client, KeyGuildChannels(guildID), channels)
		}
	}

	return
}

// GetGuild returns the guild from guildid either from cache or api
func GetGuild(client *redis.Client, guildID string) (guild *discordgo.Guild, err error) {
	// Check cache first
	err = GetCacheDataJson(client, KeyGuild(guildID), &guild)
	if err != nil {
		guild, err = BotSession.Guild(guildID)
		if err == nil {
			SetCacheDataJsonSimple(client, KeyGuild(guildID), guild)
		}
	}

	return
}

func ParseExecuteTemplate(tmplSource string, data interface{}) (string, error) {
	return ParseExecuteTemplateFM(tmplSource, data, nil)
}

func ParseExecuteTemplateFM(tmplSource string, data interface{}, f template.FuncMap) (string, error) {
	tmpl := template.New("")
	if f != nil {
		tmpl.Funcs(f)
	}

	parsed, err := tmpl.Parse(tmplSource)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = parsed.Execute(&buf, data)
	return buf.String(), err
}

func LogGetChannel(cID string) *discordgo.Channel {
	c, err := BotSession.State.Channel(cID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving channel from state")
	}
	return c
}

// Panics if chanel is not available
func MustGetChannel(cID string) *discordgo.Channel {
	c, err := BotSession.State.Channel(cID)
	if err != nil {
		panic("Failed retrieving channel from state: " + err.Error())
	}
	return c
}

func LogGetGuild(gID string) *discordgo.Guild {
	g, err := BotSession.State.Guild(gID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving guild from state")
	}
	return g
}

// Panics if guild is not available
func MustGetGuild(gID string) *discordgo.Guild {
	g, err := BotSession.State.Guild(gID)
	if err != nil {
		panic("Failed retrieving guild from state: " + err.Error())
	}
	return g
}

func RandomAdjective() string {
	return Adjectives[rand.Intn(len(Adjectives))]
}

func GetGuildMember(s *discordgo.Session, guildID, userID string) (m *discordgo.Member, err error) {
	m, err = s.State.Member(guildID, userID)
	if err == nil {
		return
	}

	log.WithField("guild", guildID).WithField("user", userID).Info("Querying api for guild member")

	m, err = s.GuildMember(guildID, userID)
	if err != nil {
		return
	}

	m.GuildID = guildID

	err = s.State.MemberAdd(m)
	return
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
