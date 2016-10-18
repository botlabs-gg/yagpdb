package web

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/miolini/datacounter"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// Misc mw that adds some headers, (Strict-Transport-Security)
func MiscMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		// force https for a year
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		inner.ServeHTTPC(ctx, w, r)
	}

	return goji.HandlerFunc(mw)
}

// Will put a redis client in the context if available
func RedisMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		//log.Println("redis middleware")
		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		client, err := common.RedisPool.Get()
		if err != nil {
			log.WithError(err).Error("Failed retrieving client from redis pool")
			// Redis is unavailable, just server without then
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		inner.ServeHTTPC(context.WithValue(ctx, ContextKeyRedis, client), w, r)
		common.RedisPool.Put(client)
	}
	return goji.HandlerFunc(mw)
}

// Fills the template data in the context with basic data such as clientid and redirects
func BaseTemplateDataMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			inner.ServeHTTPC(ctx, w, r)
			return
		}

		baseData := map[string]interface{}{
			"ClientID": common.Conf.ClientID,
			"Host":     common.Conf.Host,
			"Version":  common.VERSION,
		}
		inner.ServeHTTPC(SetContextTemplateData(ctx, baseData), w, r)
	}
	return goji.HandlerFunc(mw)
}

// Will put a session cookie in the response if not available and discord session in the context if available
func SessionMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
			// Cookie not present, skip retrieving session
			return
		}

		redisClient := RedisClientFromContext(ctx)
		if redisClient == nil {
			// Serve without session
			return
		}

		token, err := GetAuthToken(cookie.Value, redisClient)
		if err != nil {
			// No valid session
			// TODO: Should i check for json error?
			return
		}

		session, err := discordgo.New(token.Type() + " " + token.AccessToken)
		if err != nil {
			log.WithError(err).Error("Failed initializing discord session")
			return
		}

		newCtx = context.WithValue(ctx, ContextKeyDiscordSession, session)
	}
	return goji.HandlerFunc(mw)
}

// Will not serve pages unless a session is available
// Also validates the origin header if present
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

		origin := r.Header.Get("Origin")
		if origin != "" {
			if !strings.EqualFold("https://"+common.Conf.Host, origin) {
				http.Redirect(w, r, "/?err=bad_origin", http.StatusTemporaryRedirect)
				return
			}
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
				log.WithError(err).Error("Failed getting user info from discord")
				HandleLogout(ctx, w, r)
				return
			}

			// Give the little rascal to the cache
			LogIgnoreErr(common.SetCacheDataJson(redisClient, session.Token+":user", 3600, user))
		}

		var guilds []*discordgo.UserGuild
		err = common.GetCacheDataJson(redisClient, session.Token+":guilds", &guilds)
		if err != nil {
			guilds, err = session.UserGuilds()
			if err != nil {
				log.WithError(err).Error("Failed getting user guilds")
				HandleLogout(ctx, w, r)
				return
			}

			LogIgnoreErr(common.SetCacheDataJsonSimple(redisClient, session.Token+":guilds", guilds))
		}

		wrapped, err := common.GetWrapped(guilds, redisClient)
		if err != nil {
			log.WithError(err).Error("Failed wrapping guilds")
			http.Redirect(w, r, "/?err=rediserr", http.StatusTemporaryRedirect)
			return
		}

		managedGuilds := make([]*common.WrappedGuild, 0)
		for _, g := range wrapped {
			if g.Owner || g.Permissions&discordgo.PermissionManageServer != 0 {
				managedGuilds = append(managedGuilds, g)
			}
		}

		templateData := map[string]interface{}{
			"User":          user,
			"Guilds":        wrapped,
			"ManagedGuilds": managedGuilds,
		}
		newCtx := context.WithValue(SetContextTemplateData(ctx, templateData), ContextKeyUser, user)
		newCtx = context.WithValue(newCtx, ContextKeyGuilds, guilds)

		inner.ServeHTTPC(newCtx, w, r)

	}
	return goji.HandlerFunc(mw)
}

// Makes sure the user has admin priviledges on the server
// Also sets active guild
func RequireServerAdminMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guilds := ctx.Value(ContextKeyGuilds).([]*discordgo.UserGuild)
		user := ctx.Value(ContextKeyUser).(*discordgo.User)
		guildID := pat.Param(ctx, "server")

		var guild *discordgo.UserGuild
		for _, g := range guilds {
			if g.ID == guildID && (g.Owner || g.Permissions&discordgo.PermissionManageServer != 0) {
				guild = g
				break
			}
		}

		if guild == nil {
			log.Info("User tried managing server it dosen't have admin access to", user.ID, user.Username, guildID)
			http.Redirect(w, r, "/?err=noaccess", http.StatusTemporaryRedirect)
			return
		}

		fullGuild := &discordgo.Guild{
			ID:   guild.ID,
			Name: guild.Name,
		}

		newCtx := context.WithValue(ctx, ContextKeyCurrentUserGuild, guild)
		newCtx = context.WithValue(ctx, ContextKeyCurrentGuild, fullGuild)
		newCtx = SetContextTemplateData(newCtx, map[string]interface{}{"ActiveGuild": fullGuild})

		inner.ServeHTTPC(newCtx, w, r)
	}
	return goji.HandlerFunc(mw)
}

func RequireGuildChannelsMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guild := ctx.Value(ContextKeyCurrentGuild).(*discordgo.Guild)

		channels, err := common.GetGuildChannels(RedisClientFromContext(ctx), guild.ID)
		if err != nil {
			log.WithError(err).Error("Failed retrieving channels")
			http.Redirect(w, r, "/?err=retrievingchannels", http.StatusTemporaryRedirect)
			return
		}

		guild.Channels = channels

		newCtx := context.WithValue(ctx, ContextKeyGuildChannels, channels)

		inner.ServeHTTPC(newCtx, w, r)
	}
	return goji.HandlerFunc(mw)
}

func RequireFullGuildMW(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guild := ctx.Value(ContextKeyCurrentGuild).(*discordgo.Guild)

		fullGuild, err := common.GetGuild(RedisClientFromContext(ctx), guild.ID)
		if err != nil {
			log.WithError(err).Error("Failed retrieving guild")
			http.Redirect(w, r, "/?err=errretrievingguild", http.StatusTemporaryRedirect)
			return
		}

		guild.Region = fullGuild.Region
		guild.OwnerID = fullGuild.OwnerID
		guild.Roles = fullGuild.Roles

		inner.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(mw)
}

type CustomHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{}

// A helper wrapper that renders a template
func RenderHandler(inner CustomHandlerFunc, tmpl string) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		var out interface{}
		if inner != nil {
			out = inner(ctx, w, r)
		}

		if out == nil {
			if d, ok := ctx.Value(ContextKeyTemplateData).(TemplateData); ok {
				out = d
			}
		}

		LogIgnoreErr(Templates.ExecuteTemplate(w, tmpl, out))
	}
	return goji.HandlerFunc(mw)
}

// A helper wrapper that json encodes the returned value
func APIHandler(inner CustomHandlerFunc) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		out := inner(ctx, w, r)
		if out != nil {
			LogIgnoreErr(json.NewEncoder(w).Encode(out))
		}
	}
	return goji.HandlerFunc(mw)
}

// Writes the request log into logger, returns a new middleware
func RequestLogger(logger io.Writer) func(goji.Handler) goji.Handler {

	handler := func(inner goji.Handler) goji.Handler {

		mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			counter := datacounter.NewResponseWriterCounter(w)

			defer func() {
				elapsed := time.Since(started)
				dataSent := counter.Count()

				addr := strings.SplitN(r.RemoteAddr, ":", 2)[0]

				reqLine := fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto)

				out := fmt.Sprintf("%s %f - [%s] %q 200 %d %q %q\n",
					addr, elapsed.Seconds(), started.Format("02/Jan/2006:15:04:05 -0700"), reqLine, dataSent, r.UserAgent(), r.Referer())

				// GoAccess Format:
				// log-format %h %T %^[%d:%t %^] "%r" %s %b "%u" "%R"
				// date-format %d/%b/%Y
				// time-format %H:%M:%S

				logger.Write([]byte(out))
			}()

			inner.ServeHTTPC(ctx, counter, r)

		}
		return goji.HandlerFunc(mw)
	}

	return handler
}

// Parses a form
func FormParserMW(inner goji.Handler, dst interface{}) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		_, guild, tmpl := GetBaseCPContextData(ctx)

		typ := reflect.TypeOf(dst)

		// Decode the form into the destination struct
		decoded := reflect.New(typ).Interface()
		decoder := schema.NewDecoder()
		err = decoder.Decode(decoded, r.Form)

		ok := true
		if err != nil {
			log.WithError(err).Error("Failed decoding form")
			tmpl.AddAlerts(ErrorAlert("Failed parsing form"))
			ok = false
		} else {
			// Perform validation
			ok = ValidateForm(guild, tmpl, decoded)
		}

		newCtx := context.WithValue(ctx, ContextKeyParsedForm, decoded)
		newCtx = context.WithValue(newCtx, ContextKeyFormOk, ok)
		inner.ServeHTTPC(newCtx, w, r)
	}
	return goji.HandlerFunc(mw)

}

type SimpleConfigSaver interface {
	Save(guildID string, client *redis.Client) error
}

// Uses the FormParserMW to parse and validate the form, then saves it
func SimpleConfigSaverHandler(t SimpleConfigSaver, extraHandler goji.Handler) goji.Handler {
	return FormParserMW(goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		client, g, templateData := GetBaseCPContextData(ctx)

		if extraHandler != nil {
			defer extraHandler.ServeHTTPC(ctx, w, r)
		}

		form := ctx.Value(ContextKeyParsedForm).(SimpleConfigSaver)
		ok := ctx.Value(ContextKeyFormOk).(bool)
		if !ok {
			return
		}

		err := form.Save(g.ID, client)
		if !CheckErr(templateData, err, "Failed saving config", log.Error) {
			templateData.AddAlerts(SucessAlert("Sucessfully saved! :')"))
		}
	}), t)
}
