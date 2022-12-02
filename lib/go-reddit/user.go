package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

// IsUsernameAvailable determines if the supplied username is available for registration.
func (c *Client) IsUsernameAvailable(username string) (bool, error) {
	url := fmt.Sprintf("%s/api/username_available.json?user=%s", baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	isUsernameAvailable, err := strconv.ParseBool(string(bs))
	if err != nil {
		return false, err
	}

	return isUsernameAvailable, err
}

// GetUserInfo retrieves user account information for the supplied username.
func (c *Client) GetUserInfo(username string) (*Account, error) {
	url := fmt.Sprintf("%s/user/%s/about.json", baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.userAgent)

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Kind string  `json:"kind"`
		Data Account `json:data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result.Data, nil
}
