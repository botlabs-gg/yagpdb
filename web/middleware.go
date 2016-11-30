package web

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/gorilla/schema"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/reststate"
	"github.com/jonas747/yagpdb/common"
	"github.com/miolini/datacounter"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
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

		inner.ServeHTTPC(context.WithValue(ctx, common.ContextKeyRedis, client), w, r)
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

		newCtx = context.WithValue(ctx, common.ContextKeyDiscordSession, session)
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
		newCtx := context.WithValue(SetContextTemplateData(ctx, templateData), common.ContextKeyUser, user)
		newCtx = context.WithValue(newCtx, common.ContextKeyGuilds, guilds)

		inner.ServeHTTPC(newCtx, w, r)

	}
	return goji.HandlerFunc(mw)
}

func setFullGuild(ctx context.Context, guildID string) (context.Context, error) {

	fullGuild, err := common.GetGuild(RedisClientFromContext(ctx), guildID)
	if err != nil {
		log.WithError(err).Error("Failed retrieving guild")
		return ctx, err
	}

	ctx = SetContextTemplateData(ctx, map[string]interface{}{"ActiveGuild": fullGuild})
	return context.WithValue(ctx, common.ContextKeyCurrentGuild, fullGuild), nil
}

// Sets the active guild context and template data
// It will only attempt to fetch full guild if not logged in
func ActiveServerMW(inner goji.Handler) goji.Handler {

	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		defer func() {
			inner.ServeHTTPC(ctx, w, r)
		}()

		guildID := pat.Param(ctx, "server")

		// Validate the id
		if _, err := strconv.ParseInt(guildID, 10, 64); err != nil {
			log.WithError(err).Error("GuilID is not a number")
			return
		}

		guilds, ok := ctx.Value(common.ContextKeyGuilds).([]*discordgo.UserGuild)
		if !ok {
			var err error
			ctx, err = setFullGuild(ctx, guildID)
			if err != nil {
				log.WithError(err).Error("Failed setting full guild")
			}
			log.Info("No guilds, set full guild instead of userguild")
			return
		}

		var userGuild *discordgo.UserGuild
		for _, g := range guilds {
			if g.ID == guildID {
				userGuild = g
				break
			}
		}

		if userGuild == nil {
			var err error
			ctx, err = setFullGuild(ctx, guildID)
			if err != nil {
				log.WithError(err).Error("Failed setting full guild")
			}
			log.Info("Userguild not found")
			return
		}

		fullGuild := &discordgo.Guild{
			ID:   userGuild.ID,
			Name: userGuild.Name,
		}
		ctx = context.WithValue(ctx, common.ContextKeyCurrentUserGuild, userGuild)
		ctx = context.WithValue(ctx, common.ContextKeyCurrentGuild, fullGuild)
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"ActiveGuild": fullGuild})
	}
	return goji.HandlerFunc(mw)
}

func RequireActiveServer(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if v := ctx.Value(common.ContextKeyCurrentGuild); v == nil {
			http.Redirect(w, r, "/?err=no_active_guild", http.StatusTemporaryRedirect)
			return
		}

		inner.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(mw)
}

func RequireServerAdminMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if !IsAdminCtx(ctx) {
			http.Redirect(w, r, "/?err=noaccess", http.StatusTemporaryRedirect)
			return
		}

		inner.ServeHTTPC(ctx, w, r)
	}
	return goji.HandlerFunc(mw)
}

func RequireGuildChannelsMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guild := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		channels, err := common.GetGuildChannels(RedisClientFromContext(ctx), guild.ID)
		if err != nil {
			log.WithError(err).Error("Failed retrieving channels")
			http.Redirect(w, r, "/?err=retrievingchannels", http.StatusTemporaryRedirect)
			return
		}

		guild.Channels = channels

		newCtx := context.WithValue(ctx, common.ContextKeyGuildChannels, channels)

		inner.ServeHTTPC(newCtx, w, r)
	}
	return goji.HandlerFunc(mw)
}

func RequireFullGuildMW(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		guild := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		if guild.OwnerID != "" {
			// Was already full. so this is not needed
			inner.ServeHTTPC(ctx, w, r)
			return
		}

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

func RequireBotMemberMW(inner goji.Handler) goji.Handler {
	return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		member, err := reststate.GetBotMember(pat.Param(ctx, "server"))
		if err != nil {
			log.WithError(err).Warn("FALLING BACK TO DISCORD API FOR BOT MEMBER")
			member, err = DiscordSessionFromContext(ctx).GuildMember(pat.Param(ctx, "server"), common.Conf.BotID)
			log.Println(common.Conf.BotID)
			if err != nil {
				log.WithError(err).Error("Failed retrieving bot member")
				http.Redirect(w, r, "/?err=errFailedRetrievingBotMember", http.StatusTemporaryRedirect)
				return
			}
		}
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"BotMember": member})
		ctx = context.WithValue(ctx, common.ContextKeyBotMember, member)

		defer func() {
			inner.ServeHTTPC(ctx, w, r)
		}()

		log.Println("Checking if guild is available")

		// Set highest role and combined perms
		guild := ctx.Value(common.ContextKeyCurrentGuild)
		if guild == nil {
			return
		}

		guildCast := guild.(*discordgo.Guild)
		if len(guildCast.Roles) < 1 { // Not full guild
			return
		}

		log.Println("full guild available")

		var highest *discordgo.Role
		combinedPerms := 0
		for _, role := range guildCast.Roles {
			found := false
			for _, mr := range member.Roles {
				if mr == role.ID {
					found = true
					break
				}
			}

			if !found {
				continue
			}

			combinedPerms |= role.Permissions
			if highest == nil || role.Position > highest.Position {
				highest = role
			}
		}
		ctx = context.WithValue(ctx, common.ContextKeyHighestBotRole, highest)
		ctx = context.WithValue(ctx, common.ContextKeyBotPermissions, combinedPerms)
		ctx = SetContextTemplateData(ctx, map[string]interface{}{"HighestRole": highest, "BotPermissions": combinedPerms})
	})
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
			if d, ok := ctx.Value(common.ContextKeyTemplateData).(TemplateData); ok {
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
		err = decoder.Decode(decoded, r.PostForm)

		ok := true
		if err != nil {
			log.WithError(err).Error("Failed decoding form")
			tmpl.AddAlerts(ErrorAlert("Failed parsing form"))
			ok = false
		} else {
			// Perform validation
			ok = ValidateForm(guild, tmpl, decoded)
		}

		newCtx := context.WithValue(ctx, common.ContextKeyParsedForm, decoded)
		newCtx = context.WithValue(newCtx, common.ContextKeyFormOk, ok)
		inner.ServeHTTPC(newCtx, w, r)
	}
	return goji.HandlerFunc(mw)

}

type SimpleConfigSaver interface {
	Save(client *redis.Client, guildID string) error
	Name() string // Returns this config's name, as it will be logged in the server's control panel log
}

// Uses the FormParserMW to parse and validate the form, then saves it
func SimpleConfigSaverHandler(t SimpleConfigSaver, extraHandler goji.Handler) goji.Handler {
	return FormParserMW(goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		client, g, templateData := GetBaseCPContextData(ctx)

		if extraHandler != nil {
			defer extraHandler.ServeHTTPC(ctx, w, r)
		}

		form := ctx.Value(common.ContextKeyParsedForm).(SimpleConfigSaver)
		ok := ctx.Value(common.ContextKeyFormOk).(bool)
		if !ok {
			return
		}

		err := form.Save(client, g.ID)
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

type ControllerHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) (TemplateData, error)

// Handlers can return templatedata and an erro.
// If error is not nil and publicerror it will be added as an alert,
// if error is not a publicerror it will render a error page
func ControllerHandler(f ControllerHandlerFunc, templateName string) goji.Handler {
	return RenderHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request) interface{} {

		guild, _ := ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)

		data, err := f(ctx, w, r)
		if data == nil {
			ctx, data = GetCreateTemplateData(ctx)
		}

		checkControllerError(guild, data, err)

		return data

	}, templateName)
}

// Uses the FormParserMW to parse and validate the form, then saves it
func ControllerPostHandler(mainHandler ControllerHandlerFunc, extraHandler goji.Handler, formData interface{}, logMsg string) goji.Handler {
	return FormParserMW(goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		_, g, templateData := GetBaseCPContextData(ctx)

		if extraHandler != nil {
			defer func() {
				extraHandler.ServeHTTPC(ctx, w, r)
			}()
		}

		ok := ctx.Value(common.ContextKeyFormOk).(bool)
		if !ok {
			return
		}

		data, err := mainHandler(ctx, w, r)
		if data == nil {
			data = templateData
		}
		checkControllerError(g, data, err)

		if err == nil {
			data.AddAlerts(SucessAlert("Sucessfully saved! :')"))
			user, ok := ctx.Value(common.ContextKeyUser).(*discordgo.User)
			if ok {
				go common.AddCPLogEntry(user, g.ID, logMsg)
			}
		}
	}), formData)
}

func checkControllerError(guild *discordgo.Guild, data TemplateData, err error) {
	if err == nil {
		return
	}

	if cast, ok := err.(*PublicError); ok {
		data.AddAlerts(ErrorAlert(cast.Error()))
	} else {
		data.AddAlerts(ErrorAlert("An error occured... Contact support."))
	}

	entry := log.WithError(err)

	if guild != nil {
		entry = entry.WithField("guild", guild.ID)
	}

	entry.Error("Web handler reported error")
}

var stringPerms = map[int]string{
	discordgo.PermissionReadMessages:       "Read Messages",
	discordgo.PermissionSendMessages:       "Send Messages",
	discordgo.PermissionSendTTSMessages:    "Send TTS Messages",
	discordgo.PermissionManageMessages:     "Manage Messages",
	discordgo.PermissionEmbedLinks:         "Embed Links",
	discordgo.PermissionAttachFiles:        "Attach Files",
	discordgo.PermissionReadMessageHistory: "Read Message History",
	discordgo.PermissionMentionEveryone:    "Mention Everyone",
	discordgo.PermissionVoiceConnect:       "Voice Connect",
	discordgo.PermissionVoiceSpeak:         "Voice Speak",
	discordgo.PermissionVoiceMuteMembers:   "Voice Mute Members",
	discordgo.PermissionVoiceDeafenMembers: "Voice Deafen Members",
	discordgo.PermissionVoiceMoveMembers:   "Voice Move Members",
	discordgo.PermissionVoiceUseVAD:        "Voice Use VAD",

	discordgo.PermissionCreateInstantInvite: "Create Instant Invite",
	discordgo.PermissionKickMembers:         "Kick Members",
	discordgo.PermissionBanMembers:          "Ban Members",
	discordgo.PermissionManageRoles:         "Manage Roles",
	discordgo.PermissionManageChannels:      "Manage Channels",
	discordgo.PermissionManageServer:        "Manage Server",
}

func RequirePermMW(perms ...int) func(goji.Handler) goji.Handler {
	return func(inner goji.Handler) goji.Handler {
		return goji.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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
					has += stringPerms[perm]
				} else {
					if missing != "" {
						missing += ", "
					}
					missing += stringPerms[perm]

				}
			}

			c, tmpl := GetCreateTemplateData(ctx)
			ctx = c

			if missing != "" {
				tmpl.AddAlerts(ErrorAlert("This plugin is missing the following permissions: ", missing, ", It may continue to work without the functionality that requires those permissions."))
			}
			if has != "" {
				tmpl.AddAlerts(SucessAlert("The bot has the following permissions used by this plugin: ", has))
			}

			inner.ServeHTTPC(ctx, w, r)
		})
	}
}
