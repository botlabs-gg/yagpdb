package web

import (
	"fmt"
	"github.com/fzzy/radix/redis"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
	"net/http"
)

var (
	oauthConf *oauth2.Config

	// random string for oauth2 API calls to protect against CSRF
	oauthStateString = "ismellgud"
)

func InitOauth() {
	oauthConf = &oauth2.Config{
		ClientID:     Config.ClientID,
		ClientSecret: Config.ClientSecret,
		Scopes:       []string{"identify", "guilds"},
		RedirectURL:  Config.RedirectURL,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://discordapp.com/api/oauth2/token",
			AuthURL:  "https://discordapp.com/api/oauth2/authorize",
		},
	}

}

func HandleLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	url := oauthConf.AuthCodeURL(oauthStateString, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func HandleConfirmLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("error parsing form", err)
		return
	}

	for k, v := range r.Header {
		log.Printf("[%s]: %s\n", k, v)
	}

	state := r.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := oauthConf.Exchange(ctx, code)
	if err != nil {
		fmt.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, r, "/?error=oauth2failure", http.StatusTemporaryRedirect)
		return
	}

	redisClient := ctx.Value(ContextKeyRedis).(*redis.Client)

	// Create a new session cookie cause we can
	sessionCookie := GenSessionCookie()
	http.SetCookie(w, sessionCookie)

	err = SetAuthToken(token, sessionCookie.Value, redisClient)
	if err != nil {
		log.Println("Failed setting token")
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

	redisClient.Append("SELECT", 1)
	redisClient.Append("DEL", "token:"+session)
	redisClient.Append("SELECT", 0) // Put the fucker back

	for i := 0; i < 3; i++ {
		reply := redisClient.GetReply()
		if reply.Err != nil {
			log.Println("Redis error logging out", reply.Err)
		}
	}
}
