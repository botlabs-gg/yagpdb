package common

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"math/rand"
	"strconv"
	"text/template"
	"time"
)

func KeyGuild(guildID string) string         { return "guild:" + guildID }
func KeyGuildChannels(guildID string) string { return "channels:" + guildID }

func AdminOrPerm(needed int, userID, channelID string) (bool, error) {
	perms, err := BotSession.State.UserChannelPermissions(userID, channelID)
	if err != nil {
		return false, err
	}

	if perms&needed != 0 {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}

// RefreshConnectedGuilds deletes the connected_guilds set and fill it up again
// This is incase servers are removed/bot left servers while it was offline
func RefreshConnectedGuilds(session *discordgo.Session, client *redis.Client) error {
	panic("REFRESH CONNECTED DOSEN'T WORK")
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
		log.WithError(err).WithField("channel", cID).Error("Failed retrieving channel from state")
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

type DurationFormatPrecision int

const (
	DurationPrecisionSeconds DurationFormatPrecision = iota
	DurationPrecisionMinutes
	DurationPrecisionHours
	DurationPrecisionDays
	DurationPrecisionWeeks
	DurationPrecisionYears
)

func pluralize(val int64) string {
	if val == 1 {
		return ""
	}
	return "s"
}

func HumanizeDuration(precision DurationFormatPrecision, in time.Duration) (out string) {
	seconds := int64(in.Seconds())

	if precision == DurationPrecisionSeconds {
		out = fmt.Sprintf("%d Second%s", seconds%60, pluralize(seconds%60))
	}

	if precision <= DurationPrecisionMinutes && seconds >= 60 {
		out = fmt.Sprintf("%d Minute%s, %s", (seconds/60)%60, pluralize((seconds/60)%60), out)
	}

	if precision <= DurationPrecisionHours && seconds >= 60*60 {
		out = fmt.Sprintf("%d Hour%s, %s", ((seconds/60)/60)%24, pluralize(((seconds/60)/60)%24), out)
	}

	if precision <= DurationPrecisionDays && seconds >= 60*60*24 {
		out = fmt.Sprintf("%d Day%s, %s", (((seconds/60)/60)/24)%7, pluralize((((seconds/60)/60)/24)%7), out)
	}

	if precision <= DurationPrecisionWeeks && seconds >= 60*60*24*7 {
		out = fmt.Sprintf("%d Week%s, %s", ((((seconds/60)/60)/24)/7)%52, pluralize(((((seconds/60)/60)/24)/7)%52), out)
	}

	if precision <= DurationPrecisionWeeks && seconds >= 60*60*24*365 {
		out = fmt.Sprintf("%d Year%s, %s", (((seconds/60)/60)/24)/365, pluralize((((seconds/60)/60)/24)/365), out)
	}

	return
}

func SendEmbedWithFallback(s *discordgo.Session, channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	perms, err := s.State.UserChannelPermissions(s.State.User.ID, channelID)
	if err != nil {
		return nil, err
	}

	if perms&discordgo.PermissionEmbedLinks != 0 {
		return s.ChannelMessageSendEmbed(channelID, embed)
	}

	return s.ChannelMessageSend(channelID, FallbackEmbed(embed))
}

func FallbackEmbed(embed *discordgo.MessageEmbed) string {
	body := ""

	if embed.Title != "" {
		body += embed.Title + "\n"
	}

	if embed.Description != "" {
		body += embed.Description + "\n"
	}
	if body != "" {
		body += "\n"
	}

	for _, v := range embed.Fields {
		body += fmt.Sprintf("**%s**\n%s\n\n", v.Name, v.Value)
	}
	return body + "**I have no 'embed links' permissions here, this is a fallback. it looks prettier if i have that perm :)**"
}

// CutStringShort stops a strinng at "l"-3 if it's longer than "l" and adds "..."
func CutStringShort(s string, l int) string {
	var mainBuf bytes.Buffer
	var latestBuf bytes.Buffer

	i := 0
	for _, r := range s {
		latestBuf.WriteRune(r)
		if i > 3 {
			lRune, _, _ := latestBuf.ReadRune()
			mainBuf.WriteRune(lRune)
		}

		if i >= l {
			return mainBuf.String() + "..."
		}
		i++
	}

	return mainBuf.String() + latestBuf.String()
}

type SmallModel struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func MustParseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic("Failed parsing int: " + err.Error())
	}

	return i
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
