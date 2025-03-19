package antiphishing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
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

type SinkingYachtsRecentDomainsResponse struct {
	Type    string   `json:"type"`
	Domains []string `json:"domains"`
}

var (
	phishingDomainApiBaseUrl          = "https://phish.sinking.yachts/v2"
	getAllPhishingDomainsUrl          = fmt.Sprintf("%s/%s", phishingDomainApiBaseUrl, "all")
	getRecentPhishingDomainUpdatesUrl = fmt.Sprintf("%s/%s", phishingDomainApiBaseUrl, "recent")
)

var (
	fallbackPhishingUrlCheckAPI = "https://anti-fish.bitflow.dev/check"
)

const RedisKeyPhishingDomains = "phishing_domains"
const PhishingCheckTrustRatingThreshold = 0.5

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

func getAllPhishingDomains() ([]string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", getAllPhishingDomainsUrl, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("X-identify", "YAGPDB")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	domains := make([]string, 0)
	err = json.NewDecoder(resp.Body).Decode(&domains)
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func getRecentlyUpdatedPhishingDomains(seconds uint32) ([]string, []string, error) {
	client := &http.Client{}
	reqUrl := fmt.Sprintf("%s/%v", getRecentPhishingDomainUpdatesUrl, seconds)
	req, _ := http.NewRequest("GET", reqUrl, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("X-identify", "YAGPDB")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	recentlyUpdatedDomains := make([]SinkingYachtsRecentDomainsResponse, 0)
	err = json.NewDecoder(resp.Body).Decode(&recentlyUpdatedDomains)
	if err != nil {
		return nil, nil, err
	}
	added := make([]string, 0)
	deleted := make([]string, 0)
	for _, change := range recentlyUpdatedDomains {
		if change.Type == "add" {
			added = append(added, change.Domains...)
		} else if change.Type == "delete" {
			deleted = append(deleted, change.Domains...)
		}
	}
	return added, deleted, nil
}

func updateCachedPhishingDomains(seconds uint32) ([]string, []string, error) {
	added, deleted, err := getRecentlyUpdatedPhishingDomains(seconds)
	if err != nil {
		return nil, nil, err
	}
	if len(deleted) > 0 {
		deletedArgs := append([]string{RedisKeyPhishingDomains}, deleted...)
		err = common.RedisPool.Do(radix.Cmd(nil, "SREM", deletedArgs...))
		if err != nil {
			return nil, nil, err
		}
	}
	if len(added) > 0 {
		addedArgs := append([]string{RedisKeyPhishingDomains}, added...)
		err = common.RedisPool.Do(radix.Cmd(nil, "SADD", addedArgs...))
		if err != nil {
			return nil, nil, err
		}
	}
	return added, deleted, nil
}

func cacheAllPhishingDomains() ([]string, error) {
	domains, err := getAllPhishingDomains()
	if err != nil {
		return nil, err
	}

	// clear old domains incase there was a false positive
	args := append([]string{RedisKeyPhishingDomains}, domains...)
	err = common.RedisPool.Do(radix.Cmd(nil, "DEL", RedisKeyPhishingDomains))
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

func checkCacheForPhishingDomain(link string) (bool, error) {
	isBadDomain := false
	domain := common.DomainFinderRegex.FindString(link)
	if len(domain) == 0 {
		return false, nil
	}
	domain = strings.ToLower(domain)
	err := common.RedisPool.Do(radix.FlatCmd(&isBadDomain, "SISMEMBER", RedisKeyPhishingDomains, domain))
	if err != nil {
		logrus.WithError(err).Error(`[antiphishing] failed to check for phishing domains, error from cache`)
		return false, err
	}
	return isBadDomain, nil
}

func checkRemoteForPhishingUrl(input []string) (*BitFlowAntiFishResponse, error) {
	bitflowAntifishResponse := BitFlowAntiFishResponse{}
	stringifiedUrlList := strings.Join(input, ",")
	queryBytes, _ := json.Marshal(struct {
		Message string `json:"message"`
	}{stringifiedUrlList})
	queryString := string(queryBytes)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", fallbackPhishingUrlCheckAPI, strings.NewReader(queryString))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Content-Length", strconv.Itoa(len(queryString)))
	req.Header.Add("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

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
		err = fmt.Errorf("[antiphishing] Unable to fetch data from bitflowAntiFish API, status-code %d", resp.StatusCode)
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
		isPhishingLink, err := checkCacheForPhishingDomain(link)
		if err != nil {
			return "", err
		}
		if isPhishingLink {
			return link, err
		}
	}

	//if domain in link is not in cache, query BitFlow as as fallback
	bitflowAntifishResponse, err := checkRemoteForPhishingUrl(input)
	if err != nil {
		return "", err
	}

	badDomains := make([]string, 0, len(bitflowAntifishResponse.Matches))
	if bitflowAntifishResponse.Match {
		for _, match := range bitflowAntifishResponse.Matches {
			// only flag domains which have a low trust rating, this varies between 0 and 1, 0 means high trust, 1 means no trust.
			// we use PhishingCheckTrustRatingThreshold (0.5) as cutoff to make sure false positives are ignored.
			if match.TrustRating > PhishingCheckTrustRatingThreshold {
				badDomains = append(badDomains, match.Domain)
			}
		}
		return strings.Join(badDomains, ","), err
	}
	return "", nil
}

func CheckMessageForPhishingDomains(input string) (string, error) {
	input = confusables.NormalizeQueryEncodedText(input)
	matches := common.LinkRegex.FindAllString(input, -1)
	if len(matches) < 1 {
		return "", nil
	}
	return queryPhishingLinks(matches)
}
