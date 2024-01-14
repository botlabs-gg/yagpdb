package trivia

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
)

type TriviaQuestion struct {
	Question string   `json:"question"`
	Answer   string   `json:"correct_answer"`
	Category string   `json:"category"`
	Type     string   `json:"type"`
	Options  []string `json:"incorrect_answers"`
}

type TriviaResponse struct {
	Code      int               `json:"response_code"`
	Questions []*TriviaQuestion `json:"results"`
}

func FetchQuestions(amount int) ([]*TriviaQuestion, error) {
	client := &http.Client{}
	proxy := common.ConfHttpProxy.GetString()
	if len(proxy) > 0 {
		proxyUrl, err := url.Parse(proxy)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			}
		} else {
			logger.WithError(err).Error("Invalid Proxy URL, getting questions without proxy, request maybe ratelimited")
		}
	}

	url := fmt.Sprintf("https://opentdb.com/api.php?amount=%d&encode=base64", amount)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, commands.NewPublicError("Failed Getting Questions from opentdb")
	}

	var triviaResponse TriviaResponse
	err = json.NewDecoder(resp.Body).Decode(&triviaResponse)
	if err != nil {
		return nil, err
	}

	if triviaResponse.Code != 0 {
		return nil, commands.NewPublicError("Error from Trivia API")
	}

	for _, question := range triviaResponse.Questions {
		question.Decode()
		question.Category = strings.ReplaceAll(question.Category, ": ", " - ")
		if question.Type == "boolean" {
			question.Options = []string{"True", "False"}
		} else {
			question.RandomizeOptionOrder()
		}
	}

	return triviaResponse.Questions, nil
}

func (q *TriviaQuestion) Decode() {
	q.Question, _ = common.Base64DecodeToString(q.Question)
	q.Answer, _ = common.Base64DecodeToString(q.Answer)
	q.Category, _ = common.Base64DecodeToString(q.Category)
	q.Type, _ = common.Base64DecodeToString(q.Type)
	for index, option := range q.Options {
		q.Options[index], _ = common.Base64DecodeToString(option)
	}
}

// RandomizeOptionOrder randomizes the option order and returns the result
// this also adds the answer to the list of options
func (q *TriviaQuestion) RandomizeOptionOrder() {
	cop := make([]string, len(q.Options)+1)
	copy(cop, q.Options)
	cop[len(cop)-1] = q.Answer

	rand.Shuffle(len(cop), func(i, j int) {
		cop[i], cop[j] = cop[j], cop[i]
	})

	q.Options = cop
}
