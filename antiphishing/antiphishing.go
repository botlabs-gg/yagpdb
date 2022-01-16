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

	//clear old domains incase there was a false positive
	redisDelErr := common.RedisPool.Do(radix.Cmd(nil, "DEL", "hyperfish_domains"))
	if redisDelErr != nil {
		return nil, redisDelErr
	}
	//save new domains to set
	args := append([]string{"hyperfish_domains"}, domains...)
	redisSaveErr := common.RedisPool.Do(radix.Cmd(nil, "SADD", args...))
	if redisSaveErr != nil {
		return nil, redisSaveErr
	}
	return domains, nil
}

func queryHyperFish(input string) (*bool, error) {
	isBadDomain := false
	link, err := url.Parse(input)
	if err != nil {
		logrus.WithError(err).Error(`[antiphishing] failed to parse url`)
		return nil, err
	}
	redisErr := common.RedisPool.Do(radix.FlatCmd(&isBadDomain, "SISMEMBER", "hyperfish_domains", link.Hostname()))
	if redisErr != nil {
		logrus.WithError(redisErr).Error(`[antiphishing] failed to check for hyperfish_domains`)
		return nil, redisErr
	}
	return &isBadDomain, redisErr
}

func queryBitflowAntiFish(input string) (*bool, error) {
	bitflowAntifishResponse := BitFlowAntiFishResponse{}
	queryBytes, _ := json.Marshal(struct {
		Message string `json:"message"`
	}{input})
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
		return &bitflowAntifishResponse.Match, nil
	}

	if resp.StatusCode != 200 {
		respError := fmt.Errorf("[antiphishing] Unable to fetch data from AntiFish API, status-code %d", resp.StatusCode)
		logrus.WithError(respError)
		return nil, respError
	}

	req.Body.Close()
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("[antiphishing] Error parsing response body from bitflowAntiFish API")
		return nil, err
	}

	jsonErr := json.Unmarshal(bytes, &bitflowAntifishResponse)
	if err != nil {
		logrus.WithError(jsonErr).Error(("[antiphishing] Error parsing JSON from bitflowAntiFish API"))
		return nil, jsonErr
	}

	return &bitflowAntifishResponse.Match, nil
}

func checkPhishingDomains(input []string) (*string, error) {
	i := 0
	for range input {
		isPhishingLink, err := queryHyperFish(input[i])
		if err != nil {
			return nil, err
		}
		if *isPhishingLink {
			return &input[i], err
		}
		isPhishingLink, err = queryBitflowAntiFish(input[i])
		if err != nil {
			return nil, err
		}
		if *isPhishingLink {
			return &input[i], err
		}
	}
	return nil, nil
}

func CheckMessageForPhishingDomains(input string) (*string, error) {
	matches := common.LinkRegex.FindAllString(input, -1)
	if len(matches) < 1 {
		return nil, nil
	}

	return checkPhishingDomains(matches)
}
