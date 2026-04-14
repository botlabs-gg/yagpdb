package howlongtobeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/jarowinkler"
)

const (
	hltbBaseURL     = "https://howlongtobeat.com"
	hltbFindInitURL = hltbBaseURL + "/api/find/init"
	hltbFindURL     = hltbBaseURL + "/api/find"
)

func hltbClient() *http.Client {
	client := &http.Client{Timeout: 20 * time.Second}
	proxy := common.ConfHttpProxy.GetString()
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}
	return client
}

type hltbFindInitResponse struct {
	Token string `json:"token"`
	HPKey string `json:"hpKey"`
	HPVal string `json:"hpVal"`
}

type hltbSearchResponse struct {
	Data []hltbSearchGame `json:"data"`
}

type hltbSearchGame struct {
	GameID    int64   `json:"game_id"`
	GameName  string  `json:"game_name"`
	GameImage string  `json:"game_image"`
	CompMain  float64 `json:"comp_main"`
	CompPlus  float64 `json:"comp_plus"`
	CompAll   float64 `json:"comp_all"`
	Comp100   float64 `json:"comp_100"`
}

func getGameData(searchTitle string) ([]hltb, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	auth, err := getSearchAuth(ctx)
	if err != nil {
		return nil, err
	}

	results, err := searchGames(ctx, searchTitle, auth)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	normalizedTitle := strings.ToLower(normaliseTitle(searchTitle))
	parsed := parseGameData(results)
	for i := range parsed {
		parsed[i].Similarity = jarowinkler.Similarity(
			[]rune(normalizedTitle),
			[]rune(strings.ToLower(normaliseTitle(parsed[i].GameTitle))),
		)
	}

	sort.SliceStable(parsed, func(i, j int) bool {
		return parsed[i].Similarity > parsed[j].Similarity
	})

	return parsed, nil
}

func getSearchAuth(ctx context.Context) (*hltbFindInitResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hltbFindInitURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build find init request: %w", err)
	}

	q := req.URL.Query()
	q.Set("t", fmt.Sprintf("%d", time.Now().UnixMilli()))
	req.URL.RawQuery = q.Encode()

	setCommonHeaders(req)
	req.Header.Set("Accept", "*/*")

	resp, err := hltbClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("find init request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("find init failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var auth hltbFindInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		return nil, fmt.Errorf("decode find init response: %w", err)
	}

	if auth.Token == "" || auth.HPKey == "" || auth.HPVal == "" {
		return nil, fmt.Errorf("find init response missing auth fields")
	}

	return &auth, nil
}

func searchGames(ctx context.Context, searchTitle string, auth *hltbFindInitResponse) ([]hltbSearchGame, error) {
	payload := map[string]any{
		"searchType":  "games",
		"searchTerms": strings.Fields(searchTitle),
		"searchPage":  1,
		"size":        20,
		"searchOptions": map[string]any{
			"games": map[string]any{
				"userId":        0,
				"platform":      "",
				"sortCategory":  "popular",
				"rangeCategory": "main",
				"rangeTime": map[string]any{
					"min": nil,
					"max": nil,
				},
				"gameplay": map[string]any{
					"perspective": "",
					"flow":        "",
					"genre":       "",
					"difficulty":  "",
				},
				"rangeYear": map[string]any{
					"min": "",
					"max": "",
				},
				"modifier": "",
			},
			"users": map[string]any{
				"sortCategory": "postcount",
			},
			"lists": map[string]any{
				"sortCategory": "follows",
			},
			"filter":     "",
			"sort":       0,
			"randomizer": 0,
		},
		"useCache": true,
	}

	payload[auth.HPKey] = auth.HPVal

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal search body: %w", err)
	}

	doSearch := func(token, hpKey, hpVal string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, hltbFindURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build search request: %w", err)
		}

		setCommonHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("x-auth-token", token)
		req.Header.Set("x-hp-key", hpKey)
		req.Header.Set("x-hp-val", hpVal)

		resp, err := hltbClient().Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}

		return resp, nil
	}

	resp, err := doSearch(auth.Token, auth.HPKey, auth.HPVal)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		refreshedAuth, refreshErr := getSearchAuth(ctx)
		if refreshErr != nil {
			return nil, fmt.Errorf("search rejected and token refresh failed: %w", refreshErr)
		}

		delete(payload, auth.HPKey)
		payload[refreshedAuth.HPKey] = refreshedAuth.HPVal
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal refreshed search body: %w", err)
		}

		resp, err = doSearch(refreshedAuth.Token, refreshedAuth.HPKey, refreshedAuth.HPVal)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed hltbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	return parsed.Data, nil
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:149.0) Gecko/20100101 Firefox/149.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", hltbBaseURL+"/")
	req.Header.Set("Origin", hltbBaseURL)
}

func parseGameData(games []hltbSearchGame) []hltb {
	parsedGames := make([]hltb, 0, len(games))
	for _, game := range games {
		completionistSeconds := game.CompAll
		if completionistSeconds <= 0 {
			completionistSeconds = game.Comp100
		}

		mainStoryHours := secondsToHours(game.CompMain)
		mainExtraHours := secondsToHours(game.CompPlus)
		completionistHours := secondsToHours(completionistSeconds)

		parsedGames = append(parsedGames, hltb{
			GameTitle:               game.GameName,
			GameID:                  game.GameID,
			ImagePath:               game.GameImage,
			MainStoryTime:           int64(game.CompMain),
			MainStoryTimeHours:      fmt.Sprintf("%.2f Hours ", mainStoryHours),
			MainExtraTime:           int64(game.CompPlus),
			MainStoryExtraTimeHours: fmt.Sprintf("%.2f Hours ", mainExtraHours),
			CompletionistTime:       int64(completionistSeconds),
			CompletionistTimeHours:  fmt.Sprintf("%.2f Hours ", completionistHours),
			GameURL:                 fmt.Sprintf("%s://%s/game/%d", hltbScheme, hltbHost, game.GameID),
			ImageURL:                fmt.Sprintf("%s://%s/games/%s", hltbScheme, hltbHost, game.GameImage),
		})
	}

	return parsedGames
}

func secondsToHours(seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}

	return seconds / 3600
}
