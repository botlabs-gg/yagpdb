package reddit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// Link contains information about a link.
type Link struct {
	ApprovedBy          string  `json:"approved_by"`
	Archived            bool    `json:"archived"`
	Author              string  `json:"author"`
	AuthorFlairCSSClass string  `json:"author_flair_css_class"`
	AuthorFlairText     string  `json:"author_flair_text"`
	BannedBy            string  `json:"banned_by"`
	Clicked             bool    `json:"clicked"`
	ContestMode         bool    `json:"contest_mode"`
	Created             float64 `json:"created"`
	CreatedUtc          float64 `json:"created_utc"`
	CrosspostParent     string  `json:"crosspost_parent"`
	CrosspostParentList []*Link `json:"crosspost_parent_list"`
	Distinguished       string  `json:"distinguished"`
	Domain              string  `json:"domain"`
	Downs               int     `json:"downs"`
	// This can be a bool or a timestamp apperentely, what the fuck
	Edited            interface{}   `json:"edited"`
	Gilded            int           `json:"gilded"`
	Hidden            bool          `json:"hidden"`
	HideScore         bool          `json:"hide_score"`
	ID                string        `json:"id"`
	IsSelf            bool          `json:"is_self"`
	Likes             bool          `json:"likes"`
	LinkFlairCSSClass string        `json:"link_flair_css_class"`
	LinkFlairText     string        `json:"link_flair_text"`
	Locked            bool          `json:"locked"`
	Media             Media         `json:"media"`
	MediaEmbed        interface{}   `json:"media_embed"`
	ModReports        []interface{} `json:"mod_reports"`
	Name              string        `json:"name"`
	NumComments       int           `json:"num_comments"`
	NumReports        int           `json:"num_reports"`
	Over18            bool          `json:"over_18"`
	Permalink         string        `json:"permalink"`
	PostHint          string        `json:"post_hint"`
	Quarantine        bool          `json:"quarantine"`
	RemovalReason     interface{}   `json:"removal_reason"`
	ReportReasons     []interface{} `json:"report_reasons"`
	Saved             bool          `json:"saved"`
	Score             int           `json:"score"`
	SecureMedia       interface{}   `json:"secure_media"`
	SecureMediaEmbed  interface{}   `json:"secure_media_embed"`
	SelftextHTML      string        `json:"selftext_html"`
	Selftext          string        `json:"selftext"`
	Stickied          bool          `json:"stickied"`
	Subreddit         string        `json:"subreddit"`
	SubredditID       string        `json:"subreddit_id"`
	SuggestedSort     string        `json:"suggested_sort"`
	Thumbnail         string        `json:"thumbnail"`
	Title             string        `json:"title"`
	URL               string        `json:"url"`
	Ups               int           `json:"ups"`
	UserReports       []interface{} `json:"user_reports"`
	Visited           bool          `json:"visited"`
	IsRobotIndexable  bool          `json:"is_robot_indexable"`
	Spoiler           bool          `json:"spoiler"`
}

const linkType = "t3"

type linkListing struct {
	Kind string `json:"kind"`
	Data struct {
		Modhash  string `json:"modhash"`
		Children []struct {
			Kind string `json:"kind"`
			Data Link   `json:"data"`
		} `json:"children"`
		After  string      `json:"after"`
		Before interface{} `json:"before"`
	} `json:"data"`
}

// CommentOnLink posts a top-level comment to the given link. Requires the 'submit' OAuth scope.
func (c *Client) CommentOnLink(linkID string, text string) error {
	return c.commentOnThing(fmt.Sprintf("%s_%s", linkType, linkID), text)
}

// DeleteLink deletes a link submitted by the currently authenticated user. Requires the 'edit' OAuth scope.
func (c *Client) DeleteLink(linkID string) error {
	return c.deleteThing(fmt.Sprintf("%s_%s", linkType, linkID))
}

// EditLinkText edits the text of a self post by the currently authenticated user. Requires the 'edit' OAuth scope.
func (c *Client) EditLinkText(linkID string, text string) error {
	return c.editThingText(fmt.Sprintf("%s_%s", linkType, linkID), text)
}

// GetHotLinks retrieves a listing of hot links.
func (c *Client) GetHotLinks(subreddit string) ([]*Link, error) {
	return c.getLinks(subreddit, "hot", "", "")
}

// GetNewLinks retrieves a listing of new links.
func (c *Client) GetNewLinks(subreddit, before, after string) ([]*Link, error) {
	return c.getLinks(subreddit, "new", before, after)
}

// GetTopLinks retrieves a listing of top links.
func (c *Client) GetTopLinks(subreddit string) ([]*Link, error) {
	return c.getLinks(subreddit, "top", "", "")
}

// HideLink removes the given link from the user's default view of subreddit listings. Requires the 'report' OAuth scope.
func (c *Client) HideLink(linkID string) error {
	data := url.Values{}
	data.Set("id", fmt.Sprintf("%s_%s", linkType, linkID))
	url := fmt.Sprintf("%s/api/hide", baseAuthURL)
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

func (c *Client) getLinks(subreddit string, sort, before, after string) ([]*Link, error) {
	url := fmt.Sprintf("%s/r/%s/%s.json?limit=100&raw_json=1", baseURL, subreddit, sort)
	if before != "" {
		url += "&before=" + before
	} else if after != "" {
		url += "&after=" + after
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Response Headers for %v", req.URL)
	for k, v := range resp.Header {
		logrus.Debugf("%s:%s", k, v)
	}

	if resp.StatusCode != 200 {
		return nil, NewError(resp)
	}

	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result linkListing
	err = json.Unmarshal(d, &result)
	if err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			log.Println(string(d))
			log.Printf("%#v", err)
		} else {
			return nil, err
		}
	}

	var links []*Link
	for _, link := range result.Data.Children {
		anotherCopy := link
		links = append(links, &anotherCopy.Data)
	}

	return links, nil
}

func (c *Client) LinksInfo(fullnames []string) ([]*Link, error) {
	uri := baseURL + "/api/info.json?"

	param := strings.Join(fullnames, ",")
	uri += "id=" + param
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, NewError(resp)
	}

	defer resp.Body.Close()

	logrus.Debugf("Response Headers for %v", req.URL)
	for k, v := range resp.Header {
		logrus.Debugf("%s:%s", k, v)
	}
	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result linkListing
	err = json.Unmarshal(d, &result)
	if err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			log.Println(string(d))
			log.Printf("%#v", err)
		} else {
			return nil, err
		}
	}

	var links []*Link
	for _, link := range result.Data.Children {
		anotherCopy := link
		links = append(links, &anotherCopy.Data)
	}

	return links, nil
}
