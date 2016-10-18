package web

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
)

type ContextKey int

const (
	ContextKeyRedis ContextKey = iota
	ContextKeyDiscordSession
	ContextKeyTemplateData
	ContextKeyUser
	ContextKeyGuilds
	ContextKeyCurrentGuild
	ContextKeyCurrentUserGuild
	ContextKeyGuildChannels
	ContextKeyGuildRoles
	ContextKeyParsedForm
	ContextKeyFormOk
)

var ErrTokenExpired = errors.New("OAUTH2 Token expired")

// Retrives an oauth2 token for the session
// Returns an error if expired
func GetAuthToken(session string, redisClient *redis.Client) (t *oauth2.Token, err error) {
	raw, err := redisClient.Cmd("GET", "discord_token:"+session).Bytes()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &t)
	if err != nil {
		return nil, err
	}

	if !t.Valid() {
		redisClient.Cmd("DEL", "discord_token:"+session)
		err = ErrTokenExpired
	}
	return
}

// Puts an oauth2 token into redis and lets it expire after 24h cause
// how i do permanananas storage?
func SetAuthToken(token *oauth2.Token, session string, redisClient *redis.Client) error {
	serialized, err := json.Marshal(token)
	if err != nil {
		return err
	}

	redisClient.Append("SET", "discord_token:"+session, serialized)
	redisClient.Append("EXPIRE", "discord_token:"+session, 86400) // Expire after 24h

	_, err = common.GetRedisReplies(redisClient, 2)

	return err
}

func SetContextTemplateData(ctx context.Context, data map[string]interface{}) context.Context {
	// Check for existing data
	if val := ctx.Value(ContextKeyTemplateData); val != nil {
		cast := val.(TemplateData)
		for k, v := range data {
			cast[k] = v
		}
		return ctx
	}

	// Fallback
	return context.WithValue(ctx, ContextKeyTemplateData, TemplateData(data))
}

func DiscordSessionFromContext(ctx context.Context) *discordgo.Session {
	if val := ctx.Value(ContextKeyDiscordSession); val != nil {
		if cast, ok := val.(*discordgo.Session); ok {
			return cast
		}
	}
	return nil
}

func RedisClientFromContext(ctx context.Context) *redis.Client {
	if val := ctx.Value(ContextKeyRedis); val != nil {
		if cast, ok := val.(*redis.Client); ok {
			return cast
		}
	}

	return nil
}

func RandBase64(size int) string {
	b := make([]byte, size)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return base64.URLEncoding.EncodeToString(b)
}

func GenSessionCookie() *http.Cookie {
	data := RandBase64(32)
	cookie := &http.Cookie{
		Name:   "yagpdb-session",
		Value:  data,
		MaxAge: 86400,
		Path:   "/",
	}
	return cookie
}

func LogIgnoreErr(err error) {
	if err != nil {
		log.Error("Error:", err)
	}
}

type TemplateData map[string]interface{}

func (t TemplateData) AddAlerts(alerts ...*Alert) TemplateData {
	if t["Alerts"] == nil {
		t["Alerts"] = make([]*Alert, 0)
	}

	t["Alerts"] = append(t["Alerts"].([]*Alert), alerts...)
	return t
}

type Alert struct {
	Style   string
	Message string
}

const (
	AlertDanger  = "danger"
	AlertSuccess = "success"
	AlertInfo    = "info"
	AlertWarning = "warning"
)

func ErrorAlert(args ...interface{}) *Alert {
	return &Alert{
		Style:   AlertDanger,
		Message: fmt.Sprint(args...),
	}
}

func WarningAlert(args ...interface{}) *Alert {
	return &Alert{
		Style:   AlertWarning,
		Message: fmt.Sprint(args...),
	}
}

func SucessAlert(args ...interface{}) *Alert {
	return &Alert{
		Style:   AlertSuccess,
		Message: fmt.Sprint(args...),
	}
}

// Returns base context data for control panel plugins
func GetBaseCPContextData(ctx context.Context) (*redis.Client, *discordgo.Guild, TemplateData) {
	client := RedisClientFromContext(ctx)
	guild := ctx.Value(ContextKeyCurrentGuild).(*discordgo.Guild)
	templateData := ctx.Value(ContextKeyTemplateData).(TemplateData)

	return client, guild, templateData
}

// Returns a channel id from name, or if id is provided makes sure it's a channel inside the guild
// Throws a api request to guild/channels
func GetChannelId(name string, guildId string) (string, error) {
	channels, err := common.BotSession.GuildChannels(guildId)
	if err != nil {
		return "", err
	}

	var channel *discordgo.Channel
	for _, c := range channels {
		if c.ID == name || strings.EqualFold(name, c.Name) {
			channel = c
			break
		}
	}

	if channel == nil {
		return guildId, nil
	}

	return channel.ID, nil
}

// Checks and error and logs it aswell as adding it to the alerts
// returns true if an error occured
func CheckErr(t TemplateData, err error, errMsg string, logger func(...interface{})) bool {
	if err == nil {
		return false
	}

	if errMsg == "" {
		errMsg = err.Error()
	}

	t.AddAlerts(ErrorAlert("An Error occured: ", errMsg))

	if logger != nil {
		logger("An error occured:", err)
	}

	return true
}
