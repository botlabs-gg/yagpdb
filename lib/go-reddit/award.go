package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Award struct {
	AwardID     string      `json:"award_id"`
	Description interface{} `json:"description"`
	ID          string      `json:"id"`
	Icon40      string      `json:"icon_40"`
	Icon70      string      `json:"icon_70"`
	Name        string      `json:"name"`
	URL         interface{} `json:"url"`
}

// GetMyTrophies retrieves a list of awards for the currently authenticated user. Requires the 'identity' OAuth scope.
func (c *Client) GetMyTrophies() ([]*Award, error) {
	url := fmt.Sprintf("%s/api/v1/me/trophies", baseAuthURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, errors.New(fmt.Sprintf("HTTP Status Code: %d", resp.StatusCode))
	}
	defer resp.Body.Close()

	var trophyListing struct {
		Kind string `json:"kind"`
		Data struct {
			Trophies []*Award `json:"trophies"`
		} `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&trophyListing)
	if err != nil {
		return nil, err
	}

	return trophyListing.Data.Trophies, nil
}
