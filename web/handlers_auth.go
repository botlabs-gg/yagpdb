package web

import (
	"fmt"
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

	// reqBody, err := ioutil.ReadAll(r.Body)
	// if err != nil {
	// 	log.Println("Failed reading requestbody", err)
	// 	return
	// }

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

	// session, err := discordgo.New(token.Type() + " " + token.AccessToken)
	// if err != nil {
	// 	log.Println("Error creating session", err)
	// 	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	// 	return
	// }

	//client := oauthConf.Client(ctx, token)
	//discordgo.EndpointUserGuilds("@me")
	// resp, err := session.UserGuilds()
	// if err != nil {
	// 	log.Println("Error querying discord", err)
	// 	return
	// }

	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Println("Error reading respone", err)
	// 	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	// 	return
	// }

	// log.Println(resp)

	//fmt.Printf("Logged in as discord user: %s\n", *user.Login)

	redisClient := RedisClientFromContext(ctx)
	if redisClient == nil {
		log.Println("ERROR CONTACTING REDIS MON")
		http.Redirect(w, r, "/?error=redis", http.StatusTemporaryRedirect)
		return
	}

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
