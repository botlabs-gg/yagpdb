package patreonapi

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

const APIBase = "https://www.patreon.com/api/oauth2/v2"

const (
	// AuthorizationURL specifies Patreon's OAuth2 authorization endpoint (see https://tools.ietf.org/html/rfc6749#section-3.1).
	// See Example_refreshToken for examples.
	AuthorizationURL = "https://www.patreon.com/oauth2/authorize"

	// AccessTokenURL specifies Patreon's OAuth2 token endpoint (see https://tools.ietf.org/html/rfc6749#section-3.2).
	// See Example_refreshToken for examples.
	AccessTokenURL = "https://api.patreon.com/oauth2/token"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(client *http.Client) *Client {
	return &Client{httpClient: client}
}

func (c *Client) Get(path string, dataDst interface{}) error {
	resp, err := c.httpClient.Get(APIBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return c.reqError("Bad response code "+resp.Status, resp)
	}

	if dataDst != nil {
		fullbody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		err = json.Unmarshal(fullbody, dataDst)
		return err
	}

	return nil
}

func (c *Client) reqError(msg string, resp *http.Response) error {
	fullbody, _ := ioutil.ReadAll(resp.Body)

	return errors.New(msg + ": " + string(fullbody))
}

// Sample response with email scope for /identity?fields[user]=about,created,email,first_name,full_name,image_url,last_name,social_connections,thumb_url,url,vanity
func (c *Client) FetchUser() (r *UserResponse, err error) {
	v := url.Values(make(map[string][]string))
	v.Set("fields[user]", "about,created,email,first_name,full_name,image_url,last_name,social_connections,thumb_url,url,vanity")

	err = c.Get("/identity?"+v.Encode(), &r)
	return
}

func (c *Client) FetchCampaigns() (r *CampaignsResponse, err error) {
	err = c.Get("/campaigns", &r)
	return
}

func (c *Client) FetchMembers(campaign string, count int, cursor string) (r *MembersResponse, err error) {
	uri := "/campaigns/" + campaign + "/members?"

	v := url.Values(make(map[string][]string))
	v.Set("fields[member]", "full_name,is_follower,last_charge_date,last_charge_status,next_charge_date,lifetime_support_cents,currently_entitled_amount_cents,patron_status")
	v.Set("fields[user]", "about,created,first_name,full_name,image_url,last_name,social_connections,thumb_url,url,vanity")
	v.Set("fields[tier]", "amount_cents")
	v.Set("include", "user,currently_entitled_tiers")

	if cursor != "" {
		v.Set("page[cursor]", cursor)
	}

	if count != 0 {
		v.Set("page[count]", strconv.Itoa(count))
	}

	err = c.Get(uri+v.Encode(), &r)
	if err == nil {
		err = DecodeIncludes(r.Included)
	}
	return
}
