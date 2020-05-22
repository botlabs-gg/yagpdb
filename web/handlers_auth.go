package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/models"
	"github.com/mediocregopher/radix/v3"
	"golang.org/x/oauth2"
)

var (
	SessionCookieName = "yagpdb-session"
	OauthConf         *oauth2.Config
)

func InitOauth() {
	OauthConf = &oauth2.Config{
		ClientID:     common.ConfClientID.GetString(),
		ClientSecret: common.ConfClientSecret.GetString(),
		Scopes:       []string{"identify", "guilds"},
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://discordapp.com/api/oauth2/token",
			AuthURL:  "https://discordapp.com/api/oauth2/authorize",
		},
	}

	if https || exthttps {
		OauthConf.RedirectURL = "https://" + common.ConfHost.GetString() + "/confirm_login"
	} else {
		OauthConf.RedirectURL = "http://" + common.ConfHost.GetString() + "/confirm_login"
	}
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {

	csrfToken, err := CreateCSRFToken()
	if err != nil {
		CtxLogger(r.Context()).WithError(err).Error("Failed generating csrf token")
		return
	}

	redir := r.FormValue("goto")
	if redir != "" && strings.HasPrefix(redir, "/") {
		common.RedisPool.Do(radix.Cmd(nil, "SET", "csrf_redir:"+csrfToken, redir, "EX", "500"))
	}

	url := OauthConf.AuthCodeURL(csrfToken, oauth2.AccessTypeOnline)
	url += "&prompt=none"
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func HandleConfirmLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	state := r.FormValue("state")
	if ok, err := CheckCSRFToken(state); !ok {
		if err != nil {
			CtxLogger(ctx).WithError(err).Error("Failed validating CSRF token")
		} else {
			CtxLogger(ctx).Info("Invalid oauth state", state)
		}
		http.Redirect(w, r, "/?error=bad-csrf", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := OauthConf.Exchange(ctx, code)
	if err != nil {
		CtxLogger(ctx).WithError(err).Error("oauthConf.Exchange() failed")
		http.Redirect(w, r, "/?error=oauth2failure", http.StatusTemporaryRedirect)
		return
	}

	// Create a new session cookie cause we can
	sessionCookie, err := CreateCookieSession(token)
	if err != nil {
		CtxLogger(ctx).WithError(err).Error("Failed setting auth token")
		http.Redirect(w, r, "/?error=loginfailed", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, sessionCookie)

	var redirUrl string
	err = common.RedisPool.Do(radix.Cmd(&redirUrl, "GET", "csrf_redir:"+state))
	if err != nil {
		redirUrl = "/manage"
	} else {
		common.RedisPool.Do(radix.Cmd(nil, "DEL", "csrf_redir:"+state))
	}

	http.Redirect(w, r, redirUrl, http.StatusTemporaryRedirect)

}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

	sessionCookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return
	}

	sessionCookie.Value = "none"
	sessionCookie.Path = "/"
	http.SetCookie(w, sessionCookie)
}

// CreateCSRFToken creates a csrf token and adds it the list
func CreateCSRFToken() (string, error) {
	str := RandBase64(32)

	err := common.MultipleCmds(
		radix.Cmd(nil, "LPUSH", "csrf", str),
		radix.Cmd(nil, "LTRIM", "csrf", "0", "999"), // Store only 1000 crsf tokens, might need to be increased later
	)

	return str, err
}

// CheckCSRFToken returns true if it matched and false if not, an error if something bad happened
func CheckCSRFToken(token string) (bool, error) {
	var num int
	err := common.RedisPool.Do(radix.Cmd(&num, "LREM", "csrf", "1", token))
	if err != nil {
		return false, err
	}
	return num > 0, nil
}

var ErrNotLoggedIn = errors.New("Not logged in")

// AuthTokenFromB64 Retrives an oauth2 token from the base64 string
// Returns an error if expired
func AuthTokenFromB64(b64 string) (t *oauth2.Token, err error) {
	if b64 == "none" {
		return nil, ErrNotLoggedIn
	}

	decodedB64, err := base64.URLEncoding.DecodeString(b64)
	if err != nil {
		return nil, common.ErrWithCaller(err)
	}

	err = json.Unmarshal(decodedB64, &t)
	if err != nil {
		return nil, common.ErrWithCaller(err)
	}

	if !t.Valid() {
		return nil, ErrTokenExpired
	}

	return
}

// CreateCookieSession creates a session cookie where the value is the access token itself,
// this way we don't have to store it on our end anywhere.
func CreateCookieSession(token *oauth2.Token) (cookie *http.Cookie, err error) {

	token.RefreshToken = ""

	dataRaw, err := json.Marshal(token)
	if err != nil {
		return nil, common.ErrWithCaller(err)
	}

	b64 := base64.URLEncoding.EncodeToString(dataRaw)

	// Either the cookie expires in 30 days, or however long the validity of the token is if that is smaller than 7 days
	cookieExpirey := time.Hour * 24 * 30
	expiresFromNow := time.Until(token.Expiry)
	if expiresFromNow < time.Hour*24*7 {
		cookieExpirey = expiresFromNow
	}

	cookie = &http.Cookie{
		// The old cookie name can safely be used after the old format has been phased out (after a day in use)
		// Name:   "yagpdb-session",
		Name:   SessionCookieName,
		Value:  b64,
		MaxAge: int(cookieExpirey.Seconds()),
		Path:   "/",
	}

	return cookie, nil
}

func GetUserAccessLevel(userID int64, g *common.GuildWithConnected, config *models.CoreConfig, roleProvider func(guildID, userID int64) []int64) (hasRead bool, hasWrite bool) {
	// if they are the owner or they have manage server perms, then they have full access
	if g.Owner || g.Permissions&discordgo.PermissionManageServer == discordgo.PermissionManageServer {
		return true, true
	} else if !g.Connected {
		// otherwise if the bot is not on the guild then there's no config so no extra access control settings
		return false, false
	}

	if config.AllowNonMembersReadOnly {
		// everyone is allowed read access
		hasRead = true
	} else if userID != 0 && config.AllowAllMembersReadOnly {
		// logged in and a member of the guild
		hasRead = true
	}

	if len(config.AllowedWriteRoles) < 1 && len(config.AllowedReadOnlyRoles) < 1 {
		// no need to check the roles, nothing set up
		return
	}

	if userID == 0 {
		// not a member of the guild
		return
	}

	roles := roleProvider(g.ID, userID)

	if common.ContainsInt64SliceOneOf(roles, config.AllowedWriteRoles) {
		// the user has one of the write roles
		return true, true
	}

	if hasRead || common.ContainsInt64SliceOneOf(roles, config.AllowedReadOnlyRoles) {
		// the user has one of the read roles
		return true, false
	}

	return
}

// HasAccesstoGuildSettings retrusn true if the specified user (or 0 if not logged in or not on the server) has access
func HasAccesstoGuildSettings(userID int64, g *common.GuildWithConnected, config *models.CoreConfig, roleProvider func(guildID, userID int64) []int64, write bool) bool {
	hasRead, hasWrite := GetUserAccessLevel(userID, g, config, roleProvider)
	if hasWrite {
		return true
	} else if hasRead && !write {
		return true
	}

	return false
}
