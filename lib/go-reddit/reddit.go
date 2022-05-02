// Package reddit provides Reddit API wrapper utilities.
package reddit

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	baseAuthURL = "https://oauth.reddit.com"
	baseURL     = "https://oauth.reddit.com"
)

// Client is the client for interacting with the Reddit API.
type Client struct {
	http      *http.Client
	userAgent string
}

// NoAuthClient is the unauthenticated client for interacting with the Reddit API.
var NoAuthClient = &Client{
	http: new(http.Client),
}

func (c *Client) commentOnThing(fullname string, text string) error {
	data := url.Values{}
	data.Set("thing_id", fullname)
	data.Set("text", text)
	data.Set("api_type", "json")
	url := fmt.Sprintf("%s/api/comment", baseAuthURL)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(data.Encode()))

	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("HTTP Status Code: %d", resp.StatusCode))
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) deleteThing(fullname string) error {
	data := url.Values{}
	data.Set("id", fullname)
	url := fmt.Sprintf("%s/api/del", baseAuthURL)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(data.Encode()))

	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("HTTP Status Code: %d", resp.StatusCode))
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) editThingText(fullname string, text string) error {
	data := url.Values{}
	data.Set("thing_id", fullname)
	data.Set("text", text)
	data.Set("api_type", "json")
	url := fmt.Sprintf("%s/api/editusertext", baseAuthURL)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(data.Encode()))

	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("HTTP Status Code: %d", resp.StatusCode))
	}
	defer resp.Body.Close()

	return nil
}
