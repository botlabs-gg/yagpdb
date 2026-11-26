package safebrowsing

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/backgroundworkers"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/google/safebrowsing"
)

var remoteClient = &http.Client{Timeout: 3 * time.Second}

// CheckString checks a string against google safebrowsing for threats
// if the safebrowser is running on this process then it will perform the check instantly
// otherwise it will make a api request towards the safebrowsing proxy server (or return an error)
func CheckString(input string) (*safebrowsing.URLThreat, error) {
	return CheckStrings([]string{input})
}

// CheckStrings is CheckString for several strings, returning the first threat found in input order.
// Strings without links are skipped, and remote lookups are batched into as few requests as possible.
func CheckStrings(inputs []string) (*safebrowsing.URLThreat, error) {
	var lookups []string
	lookup := performRemoteLookup
	if SafeBrowser != nil {
		lookups = inputs
		lookup = performLocalLookup
	} else {
		for _, batch := range remoteBatches(inputs) {
			lookups = append(lookups, strings.Join(batch, "\n"))
		}
	}

	var lastErr error
	for _, input := range lookups {
		threat, err := lookup(input)
		if err != nil {
			lastErr = err
			continue
		}
		if threat != nil {
			return threat, nil
		}
	}
	return nil, lastErr
}

// remoteBatches drops strings the server would find no links in, and groups consecutive strings
// whose query unescaping succeeds or fails alike, since the server unescapes the joined body as a whole.
// LinkRegex never matches across a newline, so joining with one finds the same links.
func remoteBatches(inputs []string) [][]string {
	var batches [][]string
	lastValid := false
	for _, input := range inputs {
		if !common.LinkRegex.MatchString(confusables.NormalizeQueryEncodedText(input)) {
			continue
		}

		_, err := url.PathUnescape(input)
		valid := err == nil
		if len(batches) == 0 || valid != lastValid {
			batches = append(batches, nil)
			lastValid = valid
		}
		batches[len(batches)-1] = append(batches[len(batches)-1], input)
	}
	return batches
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

	resp, err := remoteClient.Do(req)
	if err != nil {
		return nil, errors.WithMessage(err, "httpclient.Do")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
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
