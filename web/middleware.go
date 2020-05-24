package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/miolini/datacounter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"goji.io/pat"
)

var (
	confGAID = config.RegisterOption("yagpdb.ga_id", "Google analytics id", "")
)

// Misc mw that adds some headers, (Strict-Transport-Security)
// And discards requests when shutting down
// And a logger
func MiscMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		if !IsAcceptingRequests() {
			w.Write([]byte(`{"error":"Shutting down, try again in a minute"}`))
			return
		}

		ctx := r.Context()

		// mark the request as partial
		if r.FormValue("partial") != "" {
			var tmplData TemplateData
			ctx, tmplData = GetCreateTemplateData(ctx)
			tmplData["PartialRequest"] = true
			ctx = context.WithValue(ctx, common.ContextKeyIsPartial, true)
		}

		entry := logger.WithFields(logrus.Fields{
			"ip":  GetRequestIP(r),
			"url": r.URL.Path,
		})
		ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)
		// force https for a year
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		inner.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(mw)
}

// Fills the template data in the context with basic data such as clientid and redirects
func BaseTemplateDataMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		// we store the light theme and sidebar_collapsed stuff in cookies
		lightTheme := false
		if cookie, err := r.Cookie("light_theme"); err == nil {
			if cookie.Value != "false" {
				lightTheme = true
			}
		}

		collapseSidebar := false
		if cookie, err := r.Cookie("sidebar_collapsed"); err == nil {
			if cookie.Value != "false" {
				collapseSidebar = true
			}
		}

		// set up the base template data
		baseData := map[string]interface{}{
			"RequestURI":       r.RequestURI,
			"StartedAtUnix":    StartedAt.Unix(),
			"CurrentAd":        CurrentAd,
			"LightTheme":       lightTheme,
			"SidebarCollapsed": collapseSidebar,
			"SidebarItems":     sideBarItems,
			"GAID":             confGAID.GetString(),
		}

		baseData["BaseURL"] = BaseURL()

		for k, v := range globalTemplateData {
			baseData[k] = v
		}

		inner.ServeHTTP(w, r.WithContext(SetContextTemplateData(r.Context(), baseData)))
	}

	return http.HandlerFunc(mw)
}

// SessionMiddleware retrieves a session from the request using the session cookie
// which is actually just a B64 encoded version of the oatuh2 token from discord for the user
func SessionMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer func() {
			inner.ServeHTTP(w, r.WithContext(ctx))
		}()

		// we actually store the discord oauth2 token for the user in their own browser instead of on the server
		// this way we avoid storing that sensitive information on the server, and it's tamper proof since its just a token.
		// we get all other information from discord itself (using said token)
		// (e.g you wont be able to say you're admin of any server you're not admin on... if you're a hackerboye reading this and trying to get ideas)
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			// Cookie not present, skip retrieving session
			return
		}

		token, err := AuthTokenFromB64(cookie.Value)
		if err != nil {
			if err != ErrNotLoggedIn {
				// this could really only happen if the user messes with the session token, or some other BS happens (like bad ram i guess)
				CtxLogger(r.Context()).WithError(err).Error("invalid session")
			}

			return
		}

		// construct the session from the user's (decoded) session cookie
		session, err := discordgo.New(token.Type() + " " + token.AccessToken)
		if err != nil {
			CtxLogger(r.Context()).WithError(err).Error("Failed initializing discord session")
			return
		}

		ctx = context.WithValue(ctx, common.ContextKeyDiscordSession, session)
	}
	return http.HandlerFunc(mw)
}

// RequireSessionMiddleware ensures that a session is available, and otherwise refuse to continue down the chain of handlers
// Also validates the origin header if present (on POST requests that is)
func RequireSessionMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		// Check if a session is present
		session := DiscordSessionFromContext(r.Context())
		if session == nil {
			http.Redirect(w, r, "/login?goto="+url.QueryEscape(r.RequestURI), http.StatusTemporaryRedirect)
			return
		}

		// validate the origin header (if present) for protection against CSRF attacks
		// i don't think putting in more protection against CSRF attacks is needed, considering literally every browser these days support it
		origin := r.Header.Get("Origin")
		if origin != "" {
			split := strings.SplitN(origin, ":", 3)
			hostSplit := strings.SplitN(common.ConfHost.GetString(), ":", 2)

			if len(split) < 2 || !strings.EqualFold("//"+hostSplit[0], split[1]) {
				CtxLogger(r.Context()).Error("Mismatched origin: ", hostSplit[0]+" : "+split[1])
				WriteErrorResponse(w, r, "Bad origin", http.StatusUnauthorized)
				return
			}
		}

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

func CSRFProtectionMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		// validate the origin header (if present) for protection against CSRF attacks
		// i don't think putting in more protection against CSRF attacks is needed, considering literally every browser these days support it
		origin := r.Header.Get("Origin")
		if origin != "" {
			split := strings.SplitN(origin, ":", 3)
			hostSplit := strings.SplitN(common.ConfHost.GetString(), ":", 2)

			if len(split) < 2 || !strings.EqualFold("//"+hostSplit[0], split[1]) {
				CtxLogger(r.Context()).Error("Mismatched origin: ", hostSplit[0]+" : "+split[1])
				WriteErrorResponse(w, r, "Bad origin", http.StatusUnauthorized)
				return
			}
		}

		inner.ServeHTTP(w, r)
	}

	return http.HandlerFunc(mw)
}

// UserInfoMiddleware fills the context with user information and the guilds it's on guilds if possible
func UserInfoMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := DiscordSessionFromContext(ctx)

		if session == nil {
			// we obviously need a session for this...
			inner.ServeHTTP(w, r)
			return
		}

		// retrieve user info
		var user *discordgo.User
		err := common.GetCacheDataJson(session.Token+":user", &user)
		if err != nil {
			// nothing in cache...
			user, err = session.UserMe()
			if err != nil {
				if !common.IsDiscordErr(err, discordgo.ErrCodeUnauthorized) {
					CtxLogger(r.Context()).WithError(err).Error("Failed getting user info from discord")
				}

				if r.URL.Path == "/logout" {
					inner.ServeHTTP(w, r)
					return
				}

				http.Redirect(w, r, "/logout", http.StatusTemporaryRedirect)
				return
			}

			// Give the little rascal to the cache
			LogIgnoreErr(common.SetCacheDataJson(session.Token+":user", 3600, user))
		}

		templateData := map[string]interface{}{
			"User":       user,
			"IsBotOwner": common.IsOwner(user.ID),
		}

		// update the logger with the user and update the context with all the new info
		entry := CtxLogger(ctx).WithField("u", user.ID)
		ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)
		ctx = context.WithValue(SetContextTemplateData(ctx, templateData), common.ContextKeyUser, user)

		inner.ServeHTTP(w, r.WithContext(ctx))

	}
	return http.HandlerFunc(mw)
}

// setFullGuild is a fallback in case a userguild is not available, could be the case if a bot admin is accesing a server they're not part of
func setFullGuild(ctx context.Context, guildID int64) (context.Context, error) {
	fullGuild, err := common.GetGuild(guildID)
	if err != nil {
		CtxLogger(ctx).WithError(err).Error("Failed retrieving guild")
		return ctx, err
	}

	entry := CtxLogger(ctx).WithField("g", guildID)
	ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)
	ctx = SetContextTemplateData(ctx, map[string]interface{}{"ActiveGuild": fullGuild})
	return context.WithValue(ctx, common.ContextKeyCurrentGuild, fullGuild), nil
}

func getGuild(guildID int64, ctx context.Context) (*discordgo.Guild, error) {
	guild, err := botrest.GetGuild(guildID)
	if err != nil {
		CtxLogger(ctx).WithError(err).Warn("failed getting guild from bot, querying discord api")

		guild, err = common.BotSession.Guild(guildID)
		if err != nil {
			CtxLogger(ctx).WithError(err).Warn("failed getting guild from discord fallback, nothing more we can do...")
			return nil, err
		}
	}

	return guild, nil
}

// Sets the active guild context and template data
// It will only attempt to fetch full guild if not logged in
func ActiveServerMW(inner http.Handler) http.Handler {

	mw := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			inner.ServeHTTP(w, r)
		}()
		ctx := r.Context()
		guildID, err := strconv.ParseInt(pat.Param(r, "server"), 10, 64)
		if err != nil {
			CtxLogger(ctx).WithError(err).Warn("GuilID is not a number")
			return
		}

		guild, err := getGuild(guildID, ctx)
		if err != nil {
			return
		}

		sort.Sort(dutil.Roles(guild.Roles))

		entry := CtxLogger(ctx).WithField("g", guildID)
		ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)
		ctx = context.WithValue(ctx, common.ContextKeyCurrentGuild, guild)

		ctx = SetContextTemplateData(ctx, map[string]interface{}{"ActiveGuild": guild})

		r = r.WithContext(ctx)
	}
	return http.HandlerFunc(mw)
}

// LoadCoreConfigMiddleware ensures that the core config is available
func LoadCoreConfigMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(common.ContextKeyCurrentGuild)
		if v == nil {
			http.Redirect(w, r, "/?err=no_active_guild", http.StatusTemporaryRedirect)
			return
		}

		g := v.(*discordgo.Guild)

		coreConf := common.GetCoreServerConfCached(g.ID)

		SetContextTemplateData(r.Context(), map[string]interface{}{"CoreConfig": coreConf})

		r = r.WithContext(context.WithValue(r.Context(), common.ContextKeyCoreConfig, coreConf))

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

// RequireActiveServer ensures that were accessing a guild specific page, and guild information is available (e.g a valid guild)
func RequireActiveServer(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		if v := r.Context().Value(common.ContextKeyCurrentGuild); v == nil {
			http.Redirect(w, r, "/?err=no_active_guild", http.StatusTemporaryRedirect)
			return
		}

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

// RequireServerAdminMiddleware restricts access to guild admins only (or bot admins)
func RequireServerAdminMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		if !ContextIsAdmin(r.Context()) {
			if DiscordSessionFromContext(r.Context()) == nil {
				// redirect them to log in and return here afterwards
				http.Redirect(w, r, "/login?goto="+url.QueryEscape(r.RequestURI), http.StatusTemporaryRedirect)
			} else {
				// they didn't have access and were logged in
				http.Redirect(w, r, "/?err=noaccess", http.StatusTemporaryRedirect)
			}
			return
		}

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

// RequireGuildChannelsMiddleware ensures that the channels are available for the guild were on during this request, and yes this has to be done seperately cause discord
func RequireGuildChannelsMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		guild := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		if len(guild.Channels) > 0 {
			// channels already available
			sort.Sort(dutil.Channels(guild.Channels))
			inner.ServeHTTP(w, r)
			return
		}

		channels, err := common.GetGuildChannels(guild.ID)
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed retrieving channels")
			http.Redirect(w, r, "/?err=retrievingchannels", http.StatusTemporaryRedirect)
			return
		}

		// Sort them
		sort.Sort(dutil.Channels(channels))
		guild.Channels = channels

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

// RequireBotMemberMW ensures that the bot member for the curreng guild is available, mostly used for checking the bot's roles
func RequireBotMemberMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parsedGuildID, _ := strconv.ParseInt(pat.Param(r, "server"), 10, 64)

		member, err := botrest.GetBotMember(parsedGuildID)
		if err != nil {
			CtxLogger(r.Context()).WithError(err).Warn("Failed contacting bot about bot member information, falling back to discord api for retrieving bot member")
			member, err = common.BotSession.GuildMember(parsedGuildID, common.BotUser.ID)
			if err != nil {
				CtxLogger(r.Context()).WithError(err).Error("Failed retrieving bot member")
				http.Redirect(w, r, "/?err=errFailedRetrievingBotMember", http.StatusTemporaryRedirect)
				return
			}
		}

		ctx := SetContextTemplateData(r.Context(), map[string]interface{}{"BotMember": member})
		ctx = context.WithValue(ctx, common.ContextKeyBotMember, member)

		defer func() {
			inner.ServeHTTP(w, r)
		}()

		// Set highest role and combined perms
		guild := ctx.Value(common.ContextKeyCurrentGuild)
		if guild == nil {
			return
		}

		guildCast := guild.(*discordgo.Guild)
		if len(guildCast.Roles) < 1 { // Not full guild
			return
		}

		var highest *discordgo.Role
		combinedPerms := 0
		for _, role := range guildCast.Roles {
			found := false
			if role.ID == guildCast.ID {
				found = true
			} else {
				for _, mr := range member.Roles {
					if mr == role.ID {
						found = true
						break
					}
				}
			}

			if !found {
				continue
			}

			combinedPerms |= role.Permissions
			if combinedPerms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
				combinedPerms |= discordgo.PermissionAll
			}

			if highest == nil || dutil.IsRoleAbove(role, highest) {
				highest = role
			}

		}

		ctx = context.WithValue(ctx, common.ContextKeyHighestBotRole, highest)
		ctx = context.WithValue(ctx, common.ContextKeyBotPermissions, combinedPerms)
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"HighestRole": highest, "BotPermissions": combinedPerms})
		r = r.WithContext(ctx)
	})
}

type CustomHandlerFunc func(w http.ResponseWriter, r *http.Request) interface{}

// A helper wrapper that renders a template
func RenderHandler(inner CustomHandlerFunc, tmpl string) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		alertsOnly := r.URL.Query().Get("alertsonly") == "1"

		respCode := 200
		if isPArtial := r.Context().Value(common.ContextKeyIsPartial); isPArtial != nil && isPArtial.(bool) {
			if formOK := r.Context().Value(common.ContextKeyFormOk); formOK != nil && !formOK.(bool) {
				alertsOnly = true
				respCode = 400
			}
		}

		if alertsOnly {
			w.Header().Set("Content-Type", "application/json")
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		var out interface{}
		if inner != nil {
			out = inner(w, r)
		}

		if out == nil {
			if d, ok := r.Context().Value(common.ContextKeyTemplateData).(TemplateData); ok {
				out = d
			}
		}

		w.WriteHeader(respCode)

		if !alertsOnly {
			err := Templates.ExecuteTemplate(w, tmpl, out)
			if err != nil {
				CtxLogger(r.Context()).WithError(err).Error("Failed executing template")
				return
			}
		} else {
			if outCast, ok := out.(TemplateData); ok {
				alertsInterface, ok := outCast["Alerts"]
				var alerts []*Alert
				if ok {
					alerts = alertsInterface.([]*Alert)
				}

				encoded, err := json.Marshal(alerts)
				if err != nil {
					CtxLogger(r.Context()).WithError(err).Error("Failed encoding alerts")
					return
				}

				w.Write(encoded)
			}
		}
	}
	return http.HandlerFunc(mw)
}

// A helper wrapper that json encodes the returned value
func APIHandler(inner CustomHandlerFunc) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		out := inner(w, r)

		if cast, ok := out.(error); ok {
			if cast == nil {
				out = map[string]interface{}{"ok": true}
			} else {
				if public, ok := cast.(*PublicError); ok {
					out = map[string]interface{}{"ok": false, "error": public.msg}
				} else {
					out = map[string]interface{}{"ok": false}
				}
				CtxLogger(r.Context()).WithError(cast).Error("API Error")
			}
			w.WriteHeader(http.StatusInternalServerError)
		}

		if out != nil {
			LogIgnoreErr(json.NewEncoder(w).Encode(out))
		}
	}
	return http.HandlerFunc(mw)
}

// Writes the request log into logger, returns a new middleware
func RequestLogger(logger io.Writer) func(http.Handler) http.Handler {

	handler := func(inner http.Handler) http.Handler {

		mw := func(w http.ResponseWriter, r *http.Request) {
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

			inner.ServeHTTP(counter, r)

		}
		return http.HandlerFunc(mw)
	}

	return handler
}

// Parses a form
func FormParserMW(inner http.Handler, dst interface{}) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		var err error
		if strings.Contains(r.Header.Get("content-type"), "multipart/form-data") {
			err = r.ParseMultipartForm(100000)
		} else {
			err = r.ParseForm()
		}

		if err != nil {
			panic(err)
		}

		ctx := r.Context()
		guild, tmpl := GetBaseCPContextData(ctx)

		typ := reflect.TypeOf(dst)

		// Decode the form into the destination struct
		decoded := reflect.New(typ).Interface()
		decoder := schema.NewDecoder()
		decoder.IgnoreUnknownKeys(true)
		err = decoder.Decode(decoded, r.Form)

		ok := true
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed decoding form")
			tmpl.AddAlerts(ErrorAlert("Failed parsing form"))
			ok = false
		} else {
			// Perform validation
			ok = ValidateForm(guild, tmpl, decoded)
		}

		newCtx := context.WithValue(ctx, common.ContextKeyParsedForm, decoded)
		newCtx = context.WithValue(newCtx, common.ContextKeyFormOk, ok)
		inner.ServeHTTP(w, r.WithContext(newCtx))
	}
	return http.HandlerFunc(mw)

}

type SimpleConfigSaver interface {
	Save(guildID int64) error
	Name() string // Returns this config's name, as it will be logged in the server's control panel log
}

// Uses the FormParserMW to parse and validate the form, then saves it
func SimpleConfigSaverHandler(t SimpleConfigSaver, extraHandler http.Handler) http.Handler {
	return FormParserMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		g, templateData := GetBaseCPContextData(ctx)

		if extraHandler != nil {
			defer extraHandler.ServeHTTP(w, r)
		}

		form := ctx.Value(common.ContextKeyParsedForm).(SimpleConfigSaver)
		ok := ctx.Value(common.ContextKeyFormOk).(bool)
		if !ok {
			return
		}

		err := form.Save(g.ID)
		if !CheckErr(templateData, err, "Failed saving config", CtxLogger(ctx).Error) {
			templateData.AddAlerts(SucessAlert("Sucessfully saved! :')"))
			user, ok := ctx.Value(common.ContextKeyUser).(*discordgo.User)
			if ok {
				common.AddCPLogEntry(user, g.ID, "Updated "+t.Name()+" Config.")
			}
		}
	}), t)
}

type PublicError struct {
	msg string
}

func (p *PublicError) Error() string {
	return p.msg
}

func NewPublicError(a ...interface{}) error {
	return &PublicError{fmt.Sprint(a...)}
}

type ControllerHandlerFunc func(w http.ResponseWriter, r *http.Request) (TemplateData, error)
type ControllerHandlerFuncJson func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// Handlers can return templatedata and an erro.
// If error is not nil and publicerror it will be added as an alert,
// if error is not a publicerror it will render a error page
func ControllerHandler(f ControllerHandlerFunc, templateName string) http.Handler {
	return RenderHandler(func(w http.ResponseWriter, r *http.Request) interface{} {
		ctx := r.Context()

		data, err := f(w, r)
		if data == nil {
			ctx, data = GetCreateTemplateData(ctx)
		}

		checkControllerError(ctx, data, err)

		return data

	}, templateName)
}

// Uses the FormParserMW to parse and validate the form, then saves it
func ControllerPostHandler(mainHandler ControllerHandlerFunc, extraHandler http.Handler, formData interface{}, logMsg string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		templateData := ctx.Value(common.ContextKeyTemplateData).(TemplateData)

		var g *discordgo.Guild
		if v := ctx.Value(common.ContextKeyCurrentGuild); v != nil {
			g = v.(*discordgo.Guild)
		}

		if extraHandler != nil {
			defer func() {
				extraHandler.ServeHTTP(w, r)
			}()
		}

		if formData != nil {
			ok := ctx.Value(common.ContextKeyFormOk).(bool)
			if !ok {
				return
			}
		}

		data, err := mainHandler(w, r)
		if data == nil {
			data = templateData
		}
		checkControllerError(ctx, data, err)

		// Don't display the success alert if there's an error alert displaying, that indicates a problem... :(
		hasErrorAlert := false
		alerts := data.Alerts()
		for _, v := range alerts {
			if v.Style == AlertDanger {
				hasErrorAlert = true
				break
			}
		}

		if err == nil && !hasErrorAlert {
			data.AddAlerts(SucessAlert("Success!"))
			user, ok := ctx.Value(common.ContextKeyUser).(*discordgo.User)
			if ok && logMsg != "" && g != nil {
				go common.AddCPLogEntry(user, g.ID, logMsg)
			}
		}
	})

	if formData != nil {
		return FormParserMW(handler, formData)
	}

	return handler
}

func checkControllerError(ctx context.Context, data TemplateData, err error) {
	if err == nil {
		return
	}

	if cast, ok := err.(*PublicError); ok {
		data.AddAlerts(ErrorAlert(cast.Error()))
	} else {
		data.AddAlerts(ErrorAlert("An error occured... Contact support if you're having issues."))
	}

	CtxLogger(ctx).WithError(err).Error("Web handler reported an error")
}

func RequirePermMW(perms ...int) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			permsInterface := ctx.Value(common.ContextKeyBotPermissions)
			currentPerms := 0
			if permsInterface == nil {
				logger.Warn("Requires perms but no permsinterface available")
			} else {
				currentPerms = permsInterface.(int)
			}

			has := ""
			missing := ""

			for _, perm := range perms {
				if currentPerms&perm != 0 {
					if has != "" {
						has += ", "
					}
					has += common.StringPerms[perm]
				} else {
					if missing != "" {
						missing += ", "
					}
					missing += common.StringPerms[perm]

				}
			}

			c, tmpl := GetCreateTemplateData(ctx)
			ctx = c

			if missing != "" {
				tmpl.AddAlerts(&Alert{
					Style:   AlertWarning,
					Message: fmt.Sprint("This plugin is missing the following permissions: ", missing, ", It may continue to work without the functionality that requires those permissions."),
				})
			}
			if has != "" {
				tmpl.AddAlerts(&Alert{
					Style:   AlertInfo,
					Message: fmt.Sprint("The bot has the following permissions used by this plugin: ", has),
				})
			}

			inner.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SetGuildMemberMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		defer func() { inner.ServeHTTP(w, r) }()

		guild := ContextGuild(r.Context())
		if guild == nil {
			return
		}

		ctx := r.Context()

		userI := r.Context().Value(common.ContextKeyUser)
		if userI != nil {
			user := userI.(*discordgo.User)
			results, err := botrest.GetMembers(guild.ID, user.ID)

			var m *discordgo.Member
			if len(results) > 0 {
				m = results[0]
			}

			if err != nil || m == nil {
				CtxLogger(r.Context()).WithError(err).Warn("failed retrieving member info from bot, falling back to discord api")

				// fallback to discord api
				m, err = common.BotSession.GuildMember(guild.ID, user.ID)
				if err != nil {
					CtxLogger(r.Context()).WithError(err).Warn("failed retrieving member info from discord api")
				}
			}

			if m != nil {
				// calculate permissions
				perms := discordgo.MemberPermissions(guild, nil, m)

				ctx = context.WithValue(r.Context(), common.ContextKeyUserMember, m)
				ctx = context.WithValue(ctx, common.ContextKeyMemberPermissions, perms)
			}
		}

		read, write := IsAdminRequest(ctx, r)
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"IsAdmin": read || write})
		ctx = context.WithValue(ctx, common.ContextKeyIsAdmin, read || write)

		if read && !write {
			var tmpl TemplateData
			ctx, tmpl = GetCreateTemplateData(ctx)
			tmpl.AddAlerts(WarningAlert("In read only mode, you can not change any settings."))
		}

		r = r.WithContext(ctx)
	}

	return http.HandlerFunc(mw)
}

func isStatic(r *http.Request) bool {
	if r.URL.Path == "/robots.txt" || len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
		return true
	}

	return false
}

// SkipStaticMW skips the "maybeSkip" handler if this is a static link
func SkipStaticMW(maybeSkip func(http.Handler) http.Handler, alwaysRunSuffixes ...string) func(http.Handler) http.Handler {
	return func(alwaysRun http.Handler) http.Handler {
		mw := func(w http.ResponseWriter, r *http.Request) {
			// reliable enough... *cough cough*
			if isStatic(r) {

				// in some cases (like the gzip handler) we wanna run certain middlewares on certain files
				for _, v := range alwaysRunSuffixes {
					if strings.HasSuffix(r.URL.Path, v) {
						// we got a match
						maybeSkip(alwaysRun).ServeHTTP(w, r)
						return
					}
				}

				alwaysRun.ServeHTTP(w, r)
				return
			}

			// not static, run the maybeskip handler
			maybeSkip(alwaysRun).ServeHTTP(w, r)
		}

		return http.HandlerFunc(mw)
	}
}

var pageHitsStatic = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "yagpdb_web_hits_total",
	Help: "Web hits total",
}, []string{"type"})

func addPromCountMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := "normal"
		if isStatic(r) {
			t = "static"
		}

		pageHitsStatic.With(prometheus.Labels{"type": t}).Inc()
		inner.ServeHTTP(w, r)
	})
}

// RequireBotOwnerMW requires the user to be logged in and that they're a bot owner
func RequireBotOwnerMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := r.Context().Value(common.ContextKeyUser); user != nil {
			cast := user.(*discordgo.User)
			if common.IsOwner(cast.ID) {
				inner.ServeHTTP(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusUnauthorized)
	})
}
