package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
)

type ContextKey int

const (
	ContextKeyRedis ContextKey = iota
	ContextKeyDiscordSession
	ContextKeyTemplateData
	ContextKeyUser
	ContextKeyGuilds
	ContextKeyCurrentGuild
)

var ErrTokenExpired = errors.New("OAUTH2 Token expired")

// Retrives an oauth2 token for the session
// Returns an error if expired
func GetAuthToken(session string, redisClient *redis.Client) (t *oauth2.Token, err error) {
	// We keep oauth tokens in db 1
	redisClient.Append("SELECT", 1)
	redisClient.Append("GET", "token:"+session)
	redisClient.Append("SELECT", 0) // Put the fucker back

	replies := common.GetRedisReplies(redisClient, 3)

	for _, r := range replies {
		if r.Err != nil {
			return nil, r.Err
		}
	}

	raw, err := replies[1].Bytes()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &t)
	if err != nil {
		return nil, err
	}

	if !t.Valid() {
		redisClient.Cmd("DEL", "token:"+session)
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

	cmds := []*common.RedisCmd{
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{1}},
		&common.RedisCmd{Name: "SET", Args: []interface{}{"token:" + session, serialized}},
		&common.RedisCmd{Name: "EXPIRE", Args: []interface{}{"token:" + session, 86400}},
		&common.RedisCmd{Name: "SELECT", Args: []interface{}{0}},
	}

	_, err = common.SafeRedisCommands(redisClient, cmds)
	if err != nil {
		return err
	}
	return nil
}

func SetContextTemplateData(ctx context.Context, data map[string]interface{}) context.Context {
	if val := ctx.Value(ContextKeyTemplateData); val != nil {
		cast := val.(map[string]interface{})
		for k, v := range data {
			cast[k] = v
		}
		return ctx
	}

	return context.WithValue(ctx, ContextKeyTemplateData, data)
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

func GenSessionCookie() *http.Cookie {
	b := make([]byte, 32)

	n, err := rand.Read(b)
	if n < len(b)-1 || err != nil {
		if err != nil {
			panic(err)
		} else {
			panic("n < len(b)")
		}
	}

	encoded := base64.URLEncoding.EncodeToString(b)

	cookie := &http.Cookie{
		Name:   "yagpdb-session",
		Value:  encoded,
		MaxAge: 86400,
		Path:   "/",
	}
	return cookie
}

func LogIgnoreErr(err error) {
	if err != nil {
		log.Println("Error:", err)
	}
}
