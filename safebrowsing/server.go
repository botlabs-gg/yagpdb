package safebrowsing

import (
	"encoding/json"
	"errors"
	"github.com/google/safebrowsing"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"
	"io/ioutil"
	"net/http"
	"os"
)

var SafeBrowser *safebrowsing.SafeBrowser
var restServer *http.Server

func RunServer() error {
	if SafeBrowser == nil {
		err := runDatabase()
		if err != nil {
			return err
		}
	}

	mux := goji.NewMux()
	mux.Handle(pat.Post("/checkmessage"), http.HandlerFunc(handleCheckMessage))

	srv := &http.Server{
		Handler: mux,
		Addr:    serverAddr,
	}
	restServer = srv

	go func() {
		logrus.Info("[safebrowsing] starting safebrowsing http server on ", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil {
			logrus.WithError(err).Error("[safebrowsing] failed starting safebrowsing server")
		}
	}()

	return nil
}

var ErrNoSafebrowsingAPIKey = errors.New("no safebrowsing api key provided")

func runDatabase() error {
	safebrowsingAPIKey := os.Getenv("YAGPDB_GOOGLE_SAFEBROWSING_API_KEY")
	if safebrowsingAPIKey == "" {
		return ErrNoSafebrowsingAPIKey
	}

	conf := safebrowsing.Config{
		APIKey: safebrowsingAPIKey,
		DBPath: "safebrowsing_db",
		Logger: logrus.StandardLogger().Writer(),
	}

	sb, err := safebrowsing.NewSafeBrowser(conf)
	if err != nil {
		return err
	} else {
		SafeBrowser = sb
	}

	return nil
}

func Shutdown() {
	SafeBrowser.Close()
}

func returnNoMatches(w http.ResponseWriter) {
	w.Write([]byte("[]"))
}

func handleCheckMessage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Error("[safebrowsing] failed reading request body")
		return
	}

	urlThreats, err := serverPerformLookup(string(body))
	if err != nil {
		returnNoMatches(w)
		logrus.WithError(err).Error("[safebrowsing] failed looking up urls: ", string(body))
		return
	}

	if len(urlThreats) < 1 {
		returnNoMatches(w)
		return
	}

	marshalled, err := json.Marshal(urlThreats)
	if err != nil {
		logrus.WithError(err).Error("[safebrowsing] failed writing response")
		returnNoMatches(w)
		return
	}

	w.Write(marshalled)
}

func serverPerformLookup(input string) (threats [][]safebrowsing.URLThreat, err error) {
	matches := common.LinkRegex.FindAllString(input, -1)
	if len(matches) < 1 {
		return nil, nil
	}

	urlThreats, err := SafeBrowser.LookupURLs(matches)
	if err != nil {
		return nil, err
	}

	return urlThreats, nil
}
