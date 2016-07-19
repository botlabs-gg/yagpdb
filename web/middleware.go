package web

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"net/url"
)

// Will put a redis client in the context if available
func RedisMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		//log.Println("redis middleware")
		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		client, err := RedisPool.Get()
		if err != nil {
			log.Println("Failed retrieving redis client from pool", err)
			// Redis is unavailable, just server without then
			inner.ServeHTTPC(ctx, w, r)
			return
		}
		inner.ServeHTTPC(context.WithValue(ctx, ContextKeyRedis, client), w, r)
		RedisPool.Put(client)
	}
	return goji.HandlerFunc(mw)
}

// Will put a session cookie in the response if not available and discord session in the context if available
func SessionMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		log.Println("Session middleware")
		//log.Println("Session middleware")
		var newCtx = ctx
		defer func() {
			inner.ServeHTTPC(newCtx, w, r)
		}()

		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			return
		}

		cookie, err := r.Cookie("yagpdb-session")
		if err != nil {
			cookie = GenSessionCookie()
			http.SetCookie(w, cookie)
			if Debug {
				log.Println("No session cookie")
			}
			// No OAUTH token can be tied to it because we just generated it so just serve
			return
		}

		redisClient := RedisClientFromContext(ctx)
		if redisClient == nil {
			// Serve without session
			if Debug {
				log.Println("redisclient is nil")
			}
			return
		}

		token, err := GetAuthToken(cookie.Value, redisClient)
		if err != nil {
			if Debug {
				log.Println("No oauth2 token", err)
			}
			return
		}

		session, err := discordgo.New(token.Type() + " " + token.AccessToken)
		if err != nil {
			if Debug {
				log.Println("Failed to initialize discordgo session")
			}
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
			if Debug {
				log.Println("Booted off request with invalid session on path that requires a session")
			}
			return
		}
		inner.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(mw)
}

// Fills the context with user and guilds if possible
func UserInfoMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		session := DiscordSessionFromContext(ctx)
		redisClient := RedisClientFromContext(ctx)

		if session == nil || redisClient == nil {
			// We can't find any info if a session or redis client is not avialable to just skiddadle our way out
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		var user *discordgo.User
		err := common.GetCacheDataJson(redisClient, session.Token+":user", &user)
		if err != nil {
			// nothing in cache...
			user, err = session.User("@me")
			if err != nil {
				log.Println("Failed getting user info, logging out..")
				http.Redirect(w, r, "/logout", http.StatusTemporaryRedirect)
				return
			}

			// Give the little rascal to the cache
			LogIgnoreErr(common.SetCacheDataJson(redisClient, session.Token+":user", 3600, user))
		}

		var guilds []*discordgo.Guild
		err = common.GetCacheDataJson(redisClient, session.Token+":guilds", &guilds)
		if err != nil {
			guilds, err = session.UserGuilds()
			if err != nil {
				log.Println("Failed getting user guilds, logging out..")
				http.Redirect(w, r, "/logout", http.StatusTemporaryRedirect)
				return
			}

			LogIgnoreErr(common.SetCacheDataJsonSimple(redisClient, session.Token+":guilds", guilds))
		}

		managedServers := make([]*discordgo.Guild, 0)
		for _, g := range guilds {
			if g.Owner || g.Permissions&discordgo.PermissionManageServer != 0 {
				managedServers = append(managedServers, g)
			}
		}

		templateData := map[string]interface{}{
			"user":           user,
			"guilds":         guilds,
			"managed_guilds": managedServers,
		}

		newCtx := context.WithValue(SetContextTemplateData(ctx, templateData), ContextKeyUser, user)
		newCtx = context.WithValue(newCtx, ContextKeyGuilds, guilds)

		inner.ServeHTTPC(newCtx, w, r)

	}
	return goji.HandlerFunc(mw)
}

// Makes sure the user has admin priviledges on the server
func RequireServerAdminMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guilds := ctx.Value(ContextKeyGuilds).([]*discordgo.Guild)
		user := ctx.Value(ContextKeyUser).(*discordgo.User)
		guildID := pat.Param(ctx, "server")

		var guild *discordgo.Guild
		for _, g := range guilds {
			if g.ID == guildID && (g.Owner || g.Permissions&discordgo.PermissionManageServer != 0) {
				guild = g
				break
			}
		}

		if guild == nil {
			log.Println("User tried managing server it dosen't have admin access to", user.ID, user.Username, guildID)
			http.Redirect(w, r, "/?err=noaccess", http.StatusTemporaryRedirect)
			return
		}

		inner.ServeHTTPC(SetContextTemplateData(ctx, map[string]interface{}{"current_guild": guild}), w, r)
	}
	return goji.HandlerFunc(mw)
}
