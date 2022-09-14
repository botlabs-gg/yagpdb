package howlongtobeat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/jarowinkler"
)

func getGameData(searchTitle string) ([]hltb, error) {
	reqData := &hltbRequest{
		SearchType:  "games",
		SearchTerms: []string{searchTitle},
	}
	u := &url.URL{
		Scheme: hltbScheme,
		Host:   hltbHost,
		Path:   hltbHostPath,
	}

	urlStr := u.String()
	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, commands.NewPublicError("Failed Parsing Query")
	}
	client := &http.Client{}
	r, _ := http.NewRequest("POST", urlStr, strings.NewReader(string(body)))
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Accept", "*/*")
	r.Header.Add("User-Agent", "Mozilla-YAGPDBv2")
	r.Header.Add("origin", "https://howlongtobeat.com")
	r.Header.Add("referer", "https://howlongtobeat.com/")

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, commands.NewPublicError("Unable to fetch data from howlongtobeat.com")
	}
	r.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var hltbData struct {
		Games []hltb `json:"data"`
	}
	err = json.Unmarshal(bytes, &hltbData)
	if err != nil {
		return nil, commands.NewPublicError("Error parsing response from howlongtobeat.com")
	}

	return hltbData.Games, nil
}

func parseGameData(gameName string, games []hltb) []hltb {
	var parsedGames []hltb
	var parsingGame hltb
	for _, game := range games {
		parsingGame = game
		parsingGame.MainStoryTimeHours = fmt.Sprintf("%.2f Hours ", (time.Second * time.Duration(game.MainStoryTime)).Hours())
		parsingGame.MainStoryExtraTimeHours = fmt.Sprintf("%.2f Hours ", (time.Second * time.Duration(game.MainExtraTime)).Hours())
		parsingGame.CompletionistTimeHours = fmt.Sprintf("%.2f Hours ", (time.Second * time.Duration(game.CompletionistTime)).Hours())
		parsingGame.JaroWinklerSimilarity = jarowinkler.Similarity([]rune(gameName), []rune(game.GameTitle))
		parsingGame.GameURL = fmt.Sprintf("%s://%s/game/%d", hltbScheme, hltbHost, games[0].GameID)
		parsingGame.ImageURL = fmt.Sprintf("%s://%s/games/%s", hltbScheme, hltbHost, games[0].ImagePath)
		parsedGames = append(parsedGames, parsingGame)
	}
	return parsedGames
}
