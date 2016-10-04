package web

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
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

func HandleLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	client := RedisClientFromContext(ctx)

	csrfToken, err := CreateCSRFToken(client)
	if err != nil {
		log.WithError(err).Error("Failed generating csrf token")
		return
	}

	url := oauthConf.AuthCodeURL(csrfToken, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func HandleConfirmLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	redisClient := ctx.Value(ContextKeyRedis).(*redis.Client)

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
	sessionCookie := GenSessionCookie()
	http.SetCookie(w, sessionCookie)

	err = SetAuthToken(token, sessionCookie.Value, redisClient)
	if err != nil {
		log.WithError(err).Error("Failed setting auth token")
		http.Redirect(w, r, "/?error=loginfailed", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func HandleLogout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

	sessionCookie, err := r.Cookie("yagpdb-session")
	if err != nil {
		return
	}
	session := sessionCookie.Value

	redisClient := ctx.Value(ContextKeyRedis).(*redis.Client)

	err = redisClient.Cmd("DEL", "discord_token:"+session).Err
	if err != nil {
		log.WithError(err).Error("Failed logging out")
	}
}

// Creates a csrf token and adds it the list
func CreateCSRFToken(client *redis.Client) (string, error) {
	str := RandBase64(32)

	client.Append("LPUSH", "csrf", str)
	client.Append("LTRIM", "csrf", 0, 99) // Store only 100 crsf tokens, might need to be increased later

	_, err := common.GetRedisReplies(client, 2)
	return str, err
}

// Returns true if it matched and false if not, an error if something bad happened
func CheckCSRFToken(client *redis.Client, token string) (bool, error) {
	num, err := client.Cmd("LREM", "csrf", 1, token).Int()
	if err != nil {
		return false, err
	}
	return num > 0, nil
}
