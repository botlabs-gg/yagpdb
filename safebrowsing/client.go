package safebrowsing

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/google/safebrowsing"
)

// CheckString checks a string against google safebrowsing for threats
// if the safebrowser is running on this process then it will perform the check instantly
// otherwise it will make a api request towards the safebrowsing proxy server (or return an error)
func CheckString(input string) (*safebrowsing.URLThreat, error) {
	if SafeBrowser != nil {
		return performLocalLookup(input)
	}

	return performRemoteLookup(input)
}

func performLocalLookup(input string) (*safebrowsing.URLThreat, error) {
	logger.Debug("performing local lookup")

	result, err := serverPerformLookup(input)
	if err != nil {
		return nil, err
	}

	return findThreatInResult(result), nil
}

func performRemoteLookup(input string) (*safebrowsing.URLThreat, error) {
	logger.Debug("performing remote lookup")

	bodyR := strings.NewReader(input)
	req, err := http.NewRequest("POST", "http://"+backgroundworkers.HTTPAddr.GetString()+"/safebroswing/checkmessage", bodyR)
	if err != nil {
		return nil, errors.WithMessage(err, "NewRequest")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WithMessage(err, "httpclient.Do")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithMessage(err, "ioutil.ReadAll")
	}

	var result [][]safebrowsing.URLThreat
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errors.WithMessage(err, "json.Unmarshal")
	}

	return findThreatInResult(result), nil
}

func findThreatInResult(result [][]safebrowsing.URLThreat) *safebrowsing.URLThreat {
	for _, list := range result {
		for _, threat := range list {
			t := threat
			return &t
		}
	}

	return nil
}
