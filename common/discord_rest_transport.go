package common

import (
	"github.com/Sirupsen/logrus"
	"net/http"
	"time"
)

type CustomDiscordHTTPTransport struct {
	Inner http.RoundTripper
}

func NewCustomDiscordHTTPTransport() *CustomDiscordHTTPTransport {
	return &CustomDiscordHTTPTransport{
		Inner: http.DefaultTransport,
	}
}

func (c *CustomDiscordHTTPTransport) RoundTrip(r *http.Request) (resp *http.Response, err error) {

	const maxSleep = time.Minute

	currentSleep := 100 * time.Millisecond

	for i := 0; i < 1000; i++ {
		resp, err = c.Inner.RoundTrip(r)
		if err == nil {
			return resp, err
		}

		logrus.WithError(err).WithField("uri", r.URL.String()).Error("Request failed, retrying")
		currentSleep *= 2
		if currentSleep >= maxSleep {
			currentSleep = maxSleep
		}

		time.Sleep(currentSleep)
		continue
	}

	logrus.WithError(err).WithField("uri", r.URL.String()).Error("Request exceeded 1000 retries")
	return
}
