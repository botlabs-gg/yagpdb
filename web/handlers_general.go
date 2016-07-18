package web

import (
	"github.com/nhooyr/color/log"
	"goji.io"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

func IndexHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := make(map[string]interface{})
	session := DiscordSessionFromContext(ctx)
	if session != nil {
		user, err := session.User("@me")
		if err != nil {
			log.Println("Error fetching user data", err)
		} else {
			templateData["logged_in"] = true
			templateData["user"] = user
		}
	}

	err := Templates.ExecuteTemplate(w, "index", templateData)
	if err != nil {
		log.Println("Failed executing templae", err)
	}
}

// Will not serve pages unless a session is available
func RequestLoggerMiddleware(inner goji.Handler) goji.Handler {
	mw := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		inner.ServeHTTPC(ctx, w, r)
		duration := time.Since(started)

		durColor := "%h[fgGreen]"

		if duration.Seconds() > 0.25 {
			durColor = "%h[fgYellow]"
		}

		if duration.Seconds() > 1 {
			durColor = "%h[fgBrightRed]"
		}

		log.Printf(durColor+"%s: Handled request [%5dms]: %s%r\n", started.Format(time.Stamp), int(duration.Seconds()*1000), r.RequestURI)
	}
	return goji.HandlerFunc(mw)
}
