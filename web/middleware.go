package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/bot/botrest"
	"github.com/jonas747/yagpdb/common"
	"github.com/miolini/datacounter"
	log "github.com/sirupsen/logrus"
	"goji.io/pat"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	GAID = os.Getenv("YAGPDB_GA_ID")
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

		if r.FormValue("partial") != "" {
			var tmplData TemplateData
			ctx, tmplData = GetCreateTemplateData(ctx)
			tmplData["PartialRequest"] = true
			ctx = context.WithValue(ctx, common.ContextKeyIsPartial, true)
		}

		entry := log.WithFields(log.Fields{
			"ip":  r.RemoteAddr,
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
		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			inner.ServeHTTP(w, r)
			return
		}

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

		baseData := map[string]interface{}{
			"BotRunning":       botrest.BotIsRunning(),
			"RequestURI":       r.RequestURI,
			"StartedAtUnix":    StartedAt.Unix(),
			"CurrentAd":        CurrentAd,
			"LightTheme":       lightTheme,
			"SidebarCollapsed": collapseSidebar,
			"GAID":             GAID,
		}

		if https || exthttps {
			baseData["BaseURL"] = "https://" + common.Conf.Host
		} else {
			baseData["BaseURL"] = "http://" + common.Conf.Host
		}

		for k, v := range globalTemplateData {
			baseData[k] = v
		}

		inner.ServeHTTP(w, r.WithContext(SetContextTemplateData(r.Context(), baseData)))
	}

	return http.HandlerFunc(mw)
}

// Will put a session cookie in the response if not available and discord session in the context if available
func SessionMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		//log.Println("Session middleware")
		ctx := r.Context()
		defer func() {
			inner.ServeHTTP(w, r.WithContext(ctx))
		}()

		if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/static/" {
			return
		}

		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			// Cookie not present, skip retrieving session
			return
		}

		token, err := AuthTokenFromB64(cookie.Value)
		if err != nil {
			log.Println("invalid session", err)
			// No valid session
			// TODO: Should i check for json error?
			return
		}

		session, err := discordgo.New(token.Type() + " " + token.AccessToken)
		if err != nil {
			CtxLogger(r.Context()).WithError(err).Error("Failed initializing discord session")
			return
		}

		ctx = context.WithValue(ctx, common.ContextKeyDiscordSession, session)
	}
	return http.HandlerFunc(mw)
}

// Will not serve pages unless a session is available
// Also validates the origin header if present
func RequireSessionMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		session := DiscordSessionFromContext(r.Context())
		if session == nil {
			WriteErrorResponse(w, r, "No session or session expired", http.StatusUnauthorized)
			return
		}

		origin := r.Header.Get("Origin")
		if origin != "" {
			split := strings.SplitN(origin, ":", 3)
			hostSplit := strings.SplitN(common.Conf.Host, ":", 2)

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

// Fills the context with user and guilds if possible
func UserInfoMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := DiscordSessionFromContext(ctx)

		if session == nil {
			// We can't find any info if a session or redis client is not avialable to just skiddadle our way out
			inner.ServeHTTP(w, r)
			return
		}

		var user *discordgo.User
		err := common.GetCacheDataJson(session.Token+":user", &user)
		if err != nil {
			// nothing in cache...
			user, err = session.UserMe()
			if err != nil {
				CtxLogger(r.Context()).WithError(err).Error("Failed getting user info from discord")
				HandleLogout(w, r)
				return
			}

			// Give the little rascal to the cache
			LogIgnoreErr(common.SetCacheDataJson(session.Token+":user", 3600, user))
		}

		var guilds []*discordgo.UserGuild
		err = common.GetCacheDataJson(discordgo.StrID(user.ID)+":guilds", &guilds)
		if err != nil {
			guilds, err = session.UserGuilds(100, 0, 0)
			if err != nil {
				CtxLogger(r.Context()).WithError(err).Error("Failed getting user guilds")
				HandleLogout(w, r)
				return
			}

			LogIgnoreErr(common.SetCacheDataJson(discordgo.StrID(user.ID)+":guilds", 10, guilds))
		}

		wrapped, err := common.GetWrapped(guilds)
		if err != nil {
			CtxLogger(r.Context()).WithError(err).Error("Failed wrapping guilds")
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
			"IsBotOwner":    false,
		}

		if user.ID == common.Conf.Owner {
			templateData["IsBotOwner"] = true
		}

		entry := CtxLogger(ctx).WithField("u", user.ID)
		ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)
		ctx = context.WithValue(SetContextTemplateData(ctx, templateData), common.ContextKeyUser, user)
		ctx = context.WithValue(ctx, common.ContextKeyGuilds, guilds)

		inner.ServeHTTP(w, r.WithContext(ctx))

	}
	return http.HandlerFunc(mw)
}

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

// Sets the active guild context and template data
// It will only attempt to fetch full guild if not logged in
func ActiveServerMW(inner http.Handler) http.Handler {

	mw := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			inner.ServeHTTP(w, r)
		}()
		ctx := r.Context()
		guildID, err := strconv.ParseInt(pat.Param(r, "server"), 10, 64)

		// Validate the id
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("GuilID is not a number")
			return
		}

		// Userguilds not available, fallback to full guild request
		guilds, ok := ctx.Value(common.ContextKeyGuilds).([]*discordgo.UserGuild)
		if !ok {
			var err error
			ctx, err = setFullGuild(ctx, guildID)
			if err != nil {
				CtxLogger(ctx).WithError(err).Error("Failed setting full guild")
			}
			r = r.WithContext(ctx)
			return
		}

		// Look for current guild in userguilds
		var userGuild *discordgo.UserGuild
		for _, g := range guilds {
			if g.ID == guildID {
				userGuild = g
				break
			}
		}

		entry := CtxLogger(ctx).WithField("g", guildID)
		ctx = context.WithValue(ctx, common.ContextKeyLogger, entry)

		// Fallback to full guild if userguilds if not found
		if userGuild == nil {
			var err error
			ctx, err = setFullGuild(ctx, guildID)
			if err != nil {
				CtxLogger(ctx).WithError(err).Error("Failed setting full guild")
			}

			ctx = SetContextTemplateData(ctx, map[string]interface{}{"IsAdmin": IsAdminRequest(ctx, r)})
			r = r.WithContext(ctx)
			return
		}

		fullGuild := &discordgo.Guild{
			ID:   userGuild.ID,
			Name: userGuild.Name,
			Icon: userGuild.Icon,
		}

		ctx = context.WithValue(ctx, common.ContextKeyCurrentUserGuild, userGuild)
		ctx = context.WithValue(ctx, common.ContextKeyCurrentGuild, fullGuild)
		isAdmin := IsAdminRequest(ctx, r)
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"ActiveGuild": fullGuild, "IsAdmin": isAdmin})
		r = r.WithContext(ctx)
	}
	return http.HandlerFunc(mw)
}

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

func RequireServerAdminMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminRequest(r.Context(), r) {
			http.Redirect(w, r, "/?err=noaccess", http.StatusTemporaryRedirect)
			return
		}

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

func RequireGuildChannelsMiddleware(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		guild := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		channels, err := common.GetGuildChannels(guild.ID)
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed retrieving channels")
			http.Redirect(w, r, "/?err=retrievingchannels", http.StatusTemporaryRedirect)
			return
		}

		sort.Sort(dutil.Channels(channels))
		guild.Channels = channels

		newCtx := context.WithValue(ctx, common.ContextKeyGuildChannels, channels)

		inner.ServeHTTP(w, r.WithContext(newCtx))
	}
	return http.HandlerFunc(mw)
}

func RequireFullGuildMW(inner http.Handler) http.Handler {
	mw := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		guild := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		if guild.OwnerID != 0 {
			// Was already full. so this is not needed
			inner.ServeHTTP(w, r)
			return
		}

		fullGuild, err := common.GetGuild(guild.ID)
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed retrieving guild")
			http.Redirect(w, r, "/?err=errretrievingguild", http.StatusTemporaryRedirect)
			return
		}

		sort.Sort(dutil.Roles(fullGuild.Roles))

		guild.Region = fullGuild.Region
		guild.OwnerID = fullGuild.OwnerID
		guild.Roles = fullGuild.Roles

		inner.ServeHTTP(w, r)
	}
	return http.HandlerFunc(mw)
}

func RequireBotMemberMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parsedGuildID, _ := strconv.ParseInt(pat.Param(r, "server"), 10, 64)

		member, err := botrest.GetBotMember(parsedGuildID)
		if err != nil {
			CtxLogger(r.Context()).WithError(err).Warn("FALLING BACK TO DISCORD API FOR BOT MEMBER")
			member, err = common.BotSession.GuildMember(parsedGuildID, common.Conf.BotID)
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
				CtxLogger(r.Context()).WithError(err).Warn("Failed executing template")
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
					CtxLogger(r.Context()).WithError(err).Warn("Failed encoding alerts")
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
		if !CheckErr(templateData, err, "Failed saving config", log.Error) {
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
				log.Warn("Requires perms but no permsinterface available")
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
