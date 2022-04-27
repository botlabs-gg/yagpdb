package reddit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Preferences contains a user's account preferences.
type Preferences struct {
	AffiliateLinks         bool        `json:"affiliate_links"`
	AllowClicktracking     bool        `json:"allow_clicktracking"`
	Beta                   bool        `json:"beta"`
	Clickgadget            bool        `json:"clickgadget"`
	CollapseLeftBar        bool        `json:"collapse_left_bar"`
	Compress               bool        `json:"compress"`
	CollapseReadMessages   bool        `json:"collapse_read_messages"`
	ContentLangs           []string    `json:"content_langs"`
	CredditAutorenew       bool        `json:"creddit_autorenew"`
	DefaultCommentSort     string      `json:"default_comment_sort"`
	DefaultThemeSr         interface{} `json:"default_theme_sr"`
	DomainDetails          bool        `json:"domain_details"`
	EmailMessages          bool        `json:"email_messages"`
	EnableDefaultThemes    bool        `json:"enable_default_themes"`
	ForceHTTPS             bool        `json:"force_https"`
	HideAds                bool        `json:"hide_ads"`
	HideDowns              bool        `json:"hide_downs"`
	HideFromRobots         bool        `json:"hide_from_robots"`
	HideLocationbar        bool        `json:"hide_locationbar"`
	HideUps                bool        `json:"hide_ups"`
	HighlightControversial bool        `json:"highlight_controversial"`
	HighlightNewComments   bool        `json:"highlight_new_comments"`
	IgnoreSuggestedSort    bool        `json:"ignore_suggested_sort"`
	LabelNsfw              bool        `json:"label_nsfw"`
	Lang                   string      `json:"lang"`
	LegacySearch           bool        `json:"legacy_search"`
	LiveOrangereds         bool        `json:"live_orangereds"`
	MarkMessagesRead       bool        `json:"mark_messages_read"`
	Media                  string      `json:"media"`
	MediaPreview           string      `json:"media_preview"`
	MinCommentScore        int         `json:"min_comment_score"`
	MinLinkScore           int         `json:"min_link_score"`
	MonitorMentions        bool        `json:"monitor_mentions"`
	Newwindow              bool        `json:"newwindow"`
	NoProfanity            bool        `json:"no_profanity"`
	NumComments            int         `json:"num_comments"`
	Numsites               int         `json:"numsites"`
	Organic                bool        `json:"organic"`
	Over18                 bool        `json:"over_18"`
	PrivateFeeds           bool        `json:"private_feeds"`
	PublicServerSeconds    bool        `json:"public_server_seconds"`
	PublicVotes            bool        `json:"public_votes"`
	Research               bool        `json:"research"`
	ShowFlair              bool        `json:"show_flair"`
	ShowGoldExpiration     bool        `json:"show_gold_expiration"`
	ShowLinkFlair          bool        `json:"show_link_flair"`
	ShowPromote            bool        `json:"show_promote"`
	ShowSnoovatar          bool        `json:"show_snoovatar"`
	ShowStylesheets        bool        `json:"show_stylesheets"`
	ShowTrending           bool        `json:"show_trending"`
	StoreVisits            bool        `json:"store_visits"`
	ThreadedMessages       bool        `json:"threaded_messages"`
	ThreadedModmail        bool        `json:"threaded_modmail"`
	UseGlobalDefaults      bool        `json:"use_global_defaults"`
}

// GetMyPreferences retrieves the accouunt preferences for the currently authenticated user. Requires the 'identity' OAuth scope.
func (c *Client) GetMyPreferences() (*Preferences, error) {
	url := fmt.Sprintf("%s/api/v1/me/preferences", baseAuthURL)
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

	var preferences Preferences
	err = json.NewDecoder(resp.Body).Decode(&preferences)
	if err != nil {
		return nil, err
	}

	return &preferences, nil
}

// UpdateMyPreferences updates the accouunt preferences for the currently authenticated user. Requires the 'account' OAuth scope.
func (c *Client) UpdateMyPreferences(preferences *Preferences) (*Preferences, error) {
	url := fmt.Sprintf("%s/api/v1/me/preferences", baseAuthURL)
	buffer := new(bytes.Buffer)
	json.NewEncoder(buffer).Encode(preferences)
	req, err := http.NewRequest("PATCH", url, buffer)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, errors.New(fmt.Sprintf("HTTP Status Code: %d", resp.StatusCode))
	}
	defer resp.Body.Close()

	var updatedPreferences Preferences
	err = json.NewDecoder(resp.Body).Decode(&updatedPreferences)
	if err != nil {
		return nil, err
	}

	return &updatedPreferences, nil
}
