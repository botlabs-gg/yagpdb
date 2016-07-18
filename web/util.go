package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"goji.io"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"net/url"
)

type ContextKey int

const (
	ContextKeyRedis ContextKey = iota
	ContextKeyDiscordSession
	ContextKeyTemplateData
)

var ErrTokenExpired = errors.New("OAUTH2 Token expired")

// Retrives an oauth2 token for the session
// Returns an error if expired
func GetAuthToken(session string, redisClient *redis.Client) (t *oauth2.Token, err error) {
	// We keep oauth tokens in db 1
	redisClient.Append("SELECT", 1)
	redisClient.Append("GET", "token:"+session)
	redisClient.Append("SELECT", 0) // Put the fucker back

	reply := redisClient.GetReply()
	if reply.Err != nil {
		return nil, reply.Err
	}

	reply = redisClient.GetReply()
	raw, err := reply.Bytes()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &t)
	if err != nil {
		return nil, err
	}

	reply = redisClient.GetReply() // last select
	if reply.Err != nil {
		return nil, reply.Err

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

	// We keep oauth tokens in db 1
	redisClient.Append("SELECT", 1)
	redisClient.Append("SET", "token:"+session, serialized)
	redisClient.Append("EXPIRE", "token:"+session, 86400)
	redisClient.Append("SELECT", 0) // Put the fucker back

	for i := 0; i < 4; i++ {
		reply := redisClient.GetReply()
		if reply.Err != nil {
			return err
		}
	}

	return nil
}

// Will put a redis client in the context if available
func RedisMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		client, err := redisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client from pool", err)
			// Redis is unavailable, just server without then
			inner.ServeHTTPC(ctx, w, r)
			return
		}
		inner.ServeHTTPC(context.WithValue(ctx, ContextKeyRedis, client), w, r)
		redisPool.Put(client)
	}
	return goji.HandlerFunc(mw)
}

// Will put a session cookie in the response if not available and discord session in the context if available
func SessionMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var newCtx = ctx
		defer inner.ServeHTTPC(newCtx, w, r)

		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			return
		}

		cookie, err := r.Cookie("yagpdb-session")
		if err != nil {
			cookie = GenSessionCookie()
			http.SetCookie(w, cookie)

			// No OAUTH token can be tied to it because we just generated it so just serve
			return
		}

		redisClient := RedisClientFromContext(ctx)
		if redisClient == nil {
			// Serve without session
			return
		}

		token, err := GetAuthToken(cookie.Value, redisClient)
		if err != nil {
			return
		}

		session, err := discordgo.New(token.Type() + " " + token.AccessToken)
		if err != nil {
			return
		}
		newCtx = context.WithValue(ctx, ContextKeyDiscordSession, session)
	}
	return goji.HandlerFunc(mw)
}

// Will not serve pages unless a session is available
func RequireSessionMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := DiscordSessionFromContext(ctx)
		if session == nil {
			values := url.Values{
				"error": []string{"No session"},
			}
			urlStr := values.Encode()
			http.Redirect(w, r, "/?"+urlStr, http.StatusTemporaryRedirect)
			return
		}
		inner.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(mw)
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
	}
	return cookie
}
