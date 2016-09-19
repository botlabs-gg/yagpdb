package web

import (
	"github.com/nhooyr/color/log"
	"goji.io"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

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

		tsStr := ""
		if LogRequestTimestamps {
			tsStr = started.Format(time.Stamp) + ": "
		}

		address := r.RemoteAddr
		log.Printf(durColor+"%sHandled request [%4dms] %s: %s%r\n", tsStr, int(duration.Seconds()*1000), address, r.RequestURI)
	}
	return goji.HandlerFunc(mw)
}
