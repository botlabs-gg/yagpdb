package patreon

import (
	"context"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/mxpv/patreon-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"os"
	"sync"
	"time"
)

var ActivePoller *Poller

type Poller struct {
	mu     sync.RWMutex
	config *oauth2.Config
	token  *oauth2.Token
	client *patreon.Client

	normalPatrons  []*patreon.User
	qualityPatrons []*patreon.User
}

func Run() {

	accessToken := os.Getenv("YAGPDB_PATREON_API_ACCESS_TOKEN")
	refreshToken := os.Getenv("YAGPDB_PATREON_API_REFRESH_TOKEN")
	clientID := os.Getenv("YAGPDB_PATREON_API_CLIENT_ID")
	clientSecret := os.Getenv("YAGPDB_PATREON_API_CLIENT_SECRET")

	if accessToken == "" || clientID == "" || clientSecret == "" {
		logrus.Warn("Patreon: Missing one of YAGPDB_PATREON_API_ACCESS_TOKEN, YAGPDB_PATREON_API_CLIENT_ID, YAGPDB_PATREON_API_CLIENT_SECRET, not starting patreon integration.")
		return
	}

	var storedRefreshToken string
	common.RedisPool.Do(radix.Cmd(&storedRefreshToken, "GET", "patreon_refresh_token"))

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  patreon.AuthorizationURL,
			TokenURL: patreon.AccessTokenURL,
		},
		Scopes: []string{"users", "pledges-to-me", "my-campaign"},
	}

	token := &oauth2.Token{
		AccessToken:  "",
		RefreshToken: refreshToken,
		// Must be non-nil, otherwise token will not be expired
		Expiry: time.Now().Add(-24 * time.Hour),
	}

	tc := oauth2.NewClient(context.Background(), &TokenSourceSaver{inner: config.TokenSource(context.Background(), token)})

	pClient := patreon.NewClient(tc)
	user, err := pClient.FetchUser()
	if err != nil {
		if storedRefreshToken == "" {
			logrus.WithError(err).Error("Patreon: Failed fetching current user with env var refresh token, no refresh token stored in redis, not starting patreon integration")
			return
		}

		logrus.WithError(err).Warn("Patreon: Failed fetching current user with env var refresh token, trying stored token")
		tCop := *token
		tCop.RefreshToken = storedRefreshToken

		tc = oauth2.NewClient(context.Background(), &TokenSourceSaver{inner: config.TokenSource(context.Background(), &tCop)})
		pClient = patreon.NewClient(tc)

		user, err = pClient.FetchUser()
		if err != nil {
			logrus.WithError(err).Error("Patreon: Failed fetching current user with stored token, not starting patreon integration")
			return
		}
	}

	poller := &Poller{
		config: config,
		token:  token,
		client: pClient,
	}

	ActivePoller = poller

	logrus.Info("Patreon integration activated as ", user.Data.ID, ": ", user.Data.Attributes.FullName)
	go poller.Run()
}

func (p *Poller) Run() {
	ticker := time.NewTicker(time.Minute)
	for {
		p.Poll()
		<-ticker.C
	}
}

func (p *Poller) Poll() {
	// Get your campaign data
	campaignResponse, err := p.client.FetchCampaign()
	if err != nil {
		logrus.WithError(err).Error("Patreon: Failed fetching campaign")
		return
	}

	campaignId := campaignResponse.Data[0].ID

	cursor := ""
	page := 1

	normalPatrons := make([]*patreon.User, 0, 25)
	qualityPatrons := make([]*patreon.User, 0, 25)
	for {
		pledgesResponse, err := p.client.FetchPledges(campaignId,
			patreon.WithPageSize(25),
			patreon.WithCursor(cursor))

		if err != nil {
			logrus.WithError(err).Error("Patreon: Failed fetching pledges")
			return
		}

		// Get all the users in an easy-to-lookup way
		users := make(map[string]*patreon.User)
		for _, item := range pledgesResponse.Included.Items {
			u, ok := item.(*patreon.User)
			if !ok {
				continue
			}

			users[u.ID] = u
		}

		// Loop over the pledges to get e.g. their amount and user name
		for _, pledge := range pledgesResponse.Data {
			if !pledge.Attributes.DeclinedSince.Time.IsZero() {
				continue
			}

			amount := pledge.Attributes.AmountCents
			user, ok := users[pledge.Relationships.Patron.Data.ID]
			if !ok {
				continue
			}

			if amount <= 200 {
				normalPatrons = append(normalPatrons, user)
			} else {
				qualityPatrons = append(qualityPatrons, user)
			}

			// fmt.Printf("%s is pledging %d cents\r\n", patronFullName, amount)
		}

		// Get the link to the next page of pledges
		nextLink := pledgesResponse.Links.Next
		if nextLink == "" {
			break
		}

		cursor = nextLink
		page++
	}

	// Swap the stored ones, this dosent mutate the existing returned slices so we dont have to do any copying on each request woo
	p.mu.Lock()
	p.normalPatrons = normalPatrons
	p.qualityPatrons = qualityPatrons
	p.mu.Unlock()
}

func (p *Poller) GetPatrons() (normal []*patreon.User, quality []*patreon.User) {
	p.mu.RLock()
	normal = p.normalPatrons
	quality = p.qualityPatrons
	p.mu.RUnlock()

	return
}

type TokenSourceSaver struct {
	inner            oauth2.TokenSource
	lastRefreshToken string
}

func (t *TokenSourceSaver) Token() (*oauth2.Token, error) {
	tk, err := t.inner.Token()
	if err == nil {
		if t.lastRefreshToken != tk.RefreshToken {
			logrus.Info("Patreon: New refresh token")
			common.RedisPool.Do(radix.Cmd(nil, "SET", "patreon_refresh_token", tk.RefreshToken))
			t.lastRefreshToken = tk.RefreshToken
		}
	}

	return tk, err
}
