/*
Using https://anti-fish.bitflow.dev/ API and its /check endpoint.
This service functions without restriction, query just needs to be regexp-filtered to avoid unnecessary requests.
*/

package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type AntiFish struct {
	Match   bool `json:"match"`
	Matches []struct {
		Followed    bool    `json:"followed"`
		Domain      string  `json:"domain"`
		URL         string  `json:"url"`
		Source      string  `json:"source"`
		Type        string  `json:"type"`
		TrustRating float64 `json:"trust_rating"`
	} `json:"matches"`
}

var (
	antiFishScheme   = "https"
	antiFishHost     = "anti-fish.bitflow.dev"
	antiFishURL      = fmt.Sprintf("%s://%s/", antiFishScheme, antiFishHost)
	antiFishHostPath = "check"
)

func AntiFishQuery(phisingQuery string) (*AntiFish, error) {
	antiFish := AntiFish{}
	phisingQuery = strings.Replace(phisingQuery, "\n", " ", -1)
	queryBytes, _ := json.Marshal(struct {
		Message string `json:"message"`
	}{phisingQuery})
	queryString := string(queryBytes)

	u := &url.URL{
		Scheme: antiFishScheme,
		Host:   antiFishHost,
		Path:   antiFishHostPath,
	}

	urlStr := u.String()
	client := &http.Client{}

	r, _ := http.NewRequest("POST", urlStr, strings.NewReader(queryString)) // URL-encoded payload
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Accept", "*/*")
	r.Header.Add("Content-Length", strconv.Itoa(len(queryString)))
	r.Header.Add("User-Agent", "YAGPDB")

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		antiFish.Match = false
		return &antiFish, nil
	}

	if resp.StatusCode != 200 {
		respError := fmt.Errorf("Unable to fetch data from AntiFish API, status-code %d", resp.StatusCode)
		return nil, respError
	}

	r.Body.Close()
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	stringBody := json.Unmarshal(bytes, &antiFish)
	if stringBody != nil {
		return nil, stringBody
	}

	return &antiFish, nil
}
