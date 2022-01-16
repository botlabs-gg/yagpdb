package antiphishing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/botlabs-gg/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

type Plugin struct {
	stopWorkers chan *sync.WaitGroup
}

type BitFlowAntiFishResponse struct {
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
	hyperphishURL      = "https://api.hyperphish.com/gimme-domains"
	bitflowAntiFishURL = "https://anti-fish.bitflow.dev/check"
)

const RedisKeyHyperfishDomains = "hyperfish_domains"

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{
		stopWorkers: make(chan *sync.WaitGroup),
	})
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "AntiPhishing",
		SysName:  "anti_phishing",
		Category: common.PluginCategoryModeration,
	}
}

func fetchHyperfishDomains() ([]string, error) {
	resp, err := http.Get(hyperphishURL)
	if err != nil {
		return nil, err
	}
	domains := make([]string, 0)
	err = json.NewDecoder(resp.Body).Decode(&domains)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return domains, nil
}

func saveHyperfishDomains() ([]string, error) {
	domains, err := fetchHyperfishDomains()
	if err != nil {
		return nil, err
	}

	// clear old domains incase there was a false positive
	args := append([]string{RedisKeyHyperfishDomains}, domains...)
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyHyperfishDomains))
	if err != nil {
		return nil, err
	}
	// and save new domains to redis set
	err = common.RedisPool.Do(radix.Cmd(nil, "SADD", args...))
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func queryHyperFish(input string) (bool, error) {
	isBadDomain := false
	link, err := url.Parse(input)
	if err != nil {
		logrus.WithError(err).Error(`[antiphishing] failed to parse url`)
		return false, err
	}
	err = common.RedisPool.Do(radix.FlatCmd(&isBadDomain, "SISMEMBER", RedisKeyHyperfishDomains, link.Hostname()))
	if err != nil {
		logrus.WithError(err).Error(`[antiphishing] failed to check for hyperfish_domains`)
		return false, err
	}
	return isBadDomain, nil
}

func queryBitflowAntiFish(input []string) (*BitFlowAntiFishResponse, error) {
	bitflowAntifishResponse := BitFlowAntiFishResponse{}
	stringifiedUrlList := strings.Join(input, ",")
	queryBytes, _ := json.Marshal(struct {
		Message string `json:"message"`
	}{stringifiedUrlList})
	queryString := string(queryBytes)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", bitflowAntiFishURL, strings.NewReader(queryString))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Content-Length", strconv.Itoa(len(queryString)))
	req.Header.Add("User-Agent", "YAGPDB")

	resp, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("[antiphishing] Failed checking bitflowAntiFish API ")
		return nil, err
	}

	if resp.StatusCode == 404 {
		bitflowAntifishResponse.Match = false
		return &bitflowAntifishResponse, nil
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("[antiphishing] Unable to fetch data from AntiFish API, status-code %d", resp.StatusCode)
		logrus.WithError(err)
		return nil, err
	}

	req.Body.Close()
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("[antiphishing] Error parsing response body from bitflowAntiFish API")
		return nil, err
	}

	err = json.Unmarshal(bytes, &bitflowAntifishResponse)
	if err != nil {
		logrus.WithError(err).Error(("[antiphishing] Error parsing JSON from bitflowAntiFish API"))
		return nil, err
	}

	return &bitflowAntifishResponse, nil
}

func queryPhishingLinks(input []string) (string, error) {
	for _, link := range input {
		isPhishingLink, err := queryHyperFish(link)
		if err != nil {
			return "", err
		}
		if isPhishingLink {
			return link, err
		}
	}

	//if link is not in hyperfish, query BitFlow
	bitflowAntifishResponse, err := queryBitflowAntiFish(input)
	if err != nil {
		return "", err
	}
	if bitflowAntifishResponse.Match {
		return bitflowAntifishResponse.Matches[0].URL, err
	}

	return "", nil
}

func CheckMessageForPhishingDomains(input string) (string, error) {
	matches := common.LinkRegex.FindAllString(input, -1)
	if len(matches) < 1 {
		return "", nil
	}

	return queryPhishingLinks(matches)
}
