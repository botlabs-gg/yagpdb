package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/oauth2"
	"github.com/jonas747/yagpdb/common"
	"net/http"
	"time"
)

var (
	oauthConf *oauth2.Config
)

func InitOauth() {
	oauthConf = &oauth2.Config{
		ClientID:     common.Conf.ClientID,
		ClientSecret: common.Conf.ClientSecret,
		Scopes:       []string{"identify", "guilds"},
		RedirectURL:  "https://" + common.Conf.Host + "/confirm_login",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://discordapp.com/api/oauth2/token",
			AuthURL:  "https://discordapp.com/api/oauth2/authorize",
		},
	}
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	client := RedisClientFromContext(r.Context())

	csrfToken, err := CreateCSRFToken(client)
	if err != nil {
		log.WithError(err).Error("Failed generating csrf token")
		return
	}

	redir := r.FormValue("goto")
	if redir != "" {
		client.Cmd("SET", "csrf_redir:"+csrfToken, redir)
		client.Cmd("EXPIRE", "csrf_redir:"+csrfToken, 500)
	}

	url := oauthConf.AuthCodeURL(csrfToken, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func HandleConfirmLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	redisClient := ctx.Value(common.ContextKeyRedis).(*redis.Client)

	state := r.FormValue("state")
	if ok, err := CheckCSRFToken(redisClient, state); !ok {
		if err != nil {
			log.WithError(err).Error("Failed validating CSRF token")
		} else {
			log.Info("Invalid oauth state", state)
		}
		http.Redirect(w, r, "/?error=bad-csrf", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := oauthConf.Exchange(ctx, code)
	if err != nil {
		log.WithError(err).Error("oauthConf.Exchange() failed")
		http.Redirect(w, r, "/?error=oauth2failure", http.StatusTemporaryRedirect)
		return
	}

	// Create a new session cookie cause we can
	sessionCookie, err := CreateCookieSession(token, redisClient)
	if err != nil {
		log.WithError(err).Error("Failed setting auth token")
		http.Redirect(w, r, "/?error=loginfailed", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, sessionCookie)

	redirUrl, err := redisClient.Cmd("GET", "csrf_redir:"+state).Str()
	if err != nil {
		redirUrl = "/cp"
	} else {
		redisClient.Cmd("DEL", "csrf_redir:"+state)
	}

	http.Redirect(w, r, redirUrl, http.StatusTemporaryRedirect)

}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

	sessionCookie, err := r.Cookie("yagpdb-session2")
	if err != nil {
		return
	}

	sessionCookie.Value = "none"
	sessionCookie.Path = "/"
	http.SetCookie(w, sessionCookie)
}

// CreateCSRFToken creates a csrf token and adds it the list
func CreateCSRFToken(client *redis.Client) (string, error) {
	str := RandBase64(32)

	client.Append("LPUSH", "csrf", str)
	client.Append("LTRIM", "csrf", 0, 99) // Store only 100 crsf tokens, might need to be increased later

	_, err := common.GetRedisReplies(client, 2)
	return str, err
}

// CheckCSRFToken returns true if it matched and false if not, an error if something bad happened
func CheckCSRFToken(client *redis.Client, token string) (bool, error) {
	num, err := client.Cmd("LREM", "csrf", 1, token).Int()
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
func CreateCookieSession(token *oauth2.Token, redisClient *redis.Client) (cookie *http.Cookie, err error) {

	token.RefreshToken = ""

	dataRaw, err := json.Marshal(token)
	if err != nil {
		return nil, common.ErrWithCaller(err)
	}

	b64 := base64.URLEncoding.EncodeToString(dataRaw)

	// Either the cookie expires in 7 days, or however long the validity of the token is if that is smaller than 7 days
	cookieExpirey := time.Hour * 24 * 7
	expiresFromNow := token.Expiry.Sub(time.Now())
	if expiresFromNow < time.Hour*24*7 {
		cookieExpirey = expiresFromNow
	}

	cookie = &http.Cookie{
		// The old cookie name can safely be used after the old format has been phased out (after a day in use)
		// Name:   "yagpdb-session",
		Name:   "yagpdb-session2",
		Value:  b64,
		MaxAge: int(cookieExpirey.Seconds()),
		Path:   "/",
	}

	return cookie, nil
}
