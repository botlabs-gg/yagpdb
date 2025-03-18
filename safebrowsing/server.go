package safebrowsing

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/google/safebrowsing"
	"github.com/sirupsen/logrus"
)

var SafeBrowser *safebrowsing.SafeBrowser

var ErrNoSafebrowsingAPIKey = errors.New("no safebrowsing api key provided")

var confSafebrowsingAPIKey = config.RegisterOption("yagpdb.google.safebrowsing_api_key", "Google safebrowsing API Key", "")

func runDatabase() error {
	safebrowsingAPIKey := confSafebrowsingAPIKey.GetString()
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
	input = confusables.NormalizeQueryEncodedText(input)
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
