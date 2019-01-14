package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	log "github.com/sirupsen/logrus"
	"goji.io/pattern"
	"net/http"
	"net/url"
	"strings"
)

var ErrTokenExpired = errors.New("OAUTH2 Token expired")

func SetContextTemplateData(ctx context.Context, data map[string]interface{}) context.Context {
	// Check for existing data
	if val := ctx.Value(common.ContextKeyTemplateData); val != nil {
		cast := val.(TemplateData)
		for k, v := range data {
			cast[k] = v
		}
		return ctx
	}

	// Fallback
	return context.WithValue(ctx, common.ContextKeyTemplateData, TemplateData(data))
}

func DiscordSessionFromContext(ctx context.Context) *discordgo.Session {
	if val := ctx.Value(common.ContextKeyDiscordSession); val != nil {
		if cast, ok := val.(*discordgo.Session); ok {
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

func (t TemplateData) Alerts() []*Alert {
	if v, ok := t["Alerts"]; ok {
		return v.([]*Alert)
	}

	return nil
}

func GetCreateTemplateData(ctx context.Context) (context.Context, TemplateData) {
	if v := ctx.Value(common.ContextKeyTemplateData); v != nil {
		return ctx, v.(TemplateData)
	}
	tmplData := TemplateData(make(map[string]interface{}))
	ctx = context.WithValue(ctx, common.ContextKeyTemplateData, tmplData)
	return ctx, tmplData
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

func ContextGuild(ctx context.Context) *discordgo.Guild {
	return ctx.Value(common.ContextKeyCurrentGuild).(*discordgo.Guild)
}

// Returns base context data for control panel plugins
func GetBaseCPContextData(ctx context.Context) (*discordgo.Guild, TemplateData) {
	var guild *discordgo.Guild
	if v := ctx.Value(common.ContextKeyCurrentGuild); v != nil {
		guild = v.(*discordgo.Guild)
	}

	templateData := ctx.Value(common.ContextKeyTemplateData).(TemplateData)

	return guild, templateData
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

// Checks the context if there is a logged in user and if so if he's and admin or not
func IsAdminRequest(ctx context.Context, r *http.Request) bool {
	if v := ctx.Value(common.ContextKeyCurrentUserGuild); v != nil {

		cast := v.(*discordgo.UserGuild)
		// Require manageserver, ownership of guild or ownership of bot
		if cast.Owner || cast.Permissions&discordgo.PermissionManageServer != 0 {
			return true
		}
	}

	if user := ctx.Value(common.ContextKeyUser); user != nil {
		cast := user.(*discordgo.User)
		if cast.ID == common.Conf.Owner {
			return true
		}

		if strings.EqualFold(r.Method, "GET") || strings.EqualFold(r.Method, "OPTIONS") {
			if hasAcces, err := bot.HasReadOnlyAccess(cast.ID); hasAcces && err == nil {
				return true
			}
		}
	}
	return false
}

func HasPermissionCTX(ctx context.Context, perms int) bool {
	if v := ctx.Value(common.ContextKeyCurrentUserGuild); v != nil {

		cast := v.(*discordgo.UserGuild)
		// Require manageserver, ownership of guild or ownership of bot
		if cast.Owner || cast.Permissions&discordgo.PermissionAdministrator != 0 || cast.Permissions&discordgo.PermissionManageServer != 0 || cast.Permissions&perms != 0 {
			return true
		}
	}

	return false
}

type APIError struct {
	Message string
}

// CtxLogger Returns an always non nil entry either from the context or standard logger
func CtxLogger(ctx context.Context) *log.Entry {
	if inter := ctx.Value(common.ContextKeyLogger); inter != nil {
		return inter.(*log.Entry)
	}

	return log.NewEntry(log.StandardLogger())
}

func WriteErrorResponse(w http.ResponseWriter, r *http.Request, err string, statusCode int) {
	if r.FormValue("partial") != "" {
		w.WriteHeader(statusCode)
		w.Write([]byte(`{"error": "` + err + `"}`))
		return
	}

	http.Redirect(w, r, "/?error="+url.QueryEscape(err), http.StatusTemporaryRedirect)
	return

}

func IsRequestPartial(ctx context.Context) bool {
	if v := ctx.Value(common.ContextKeyIsPartial); v != nil {
		return v.(bool)
	}

	return false
}

func ContextUser(ctx context.Context) *discordgo.User {
	return ctx.Value(common.ContextKeyUser).(*discordgo.User)
}

func ParamOrEmpty(r *http.Request, key string) string {
	s := r.Context().Value(pattern.Variable(key))
	if s != nil {
		return s.(string)
	}

	return ""
}
