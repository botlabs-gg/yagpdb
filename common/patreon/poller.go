package patreon

import (
	"context"
	"github.com/jonas747/patreon-go"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"os"
	"strconv"
	"sync"
	"time"
)

type Poller struct {
	mu     sync.RWMutex
	config *oauth2.Config
	token  *oauth2.Token
	client *patreon.Client

	activePatrons []*Patron
}

func Run() {

	accessToken := os.Getenv("YAGPDB_PATREON_API_ACCESS_TOKEN")
	refreshToken := os.Getenv("YAGPDB_PATREON_API_REFRESH_TOKEN")
	clientID := os.Getenv("YAGPDB_PATREON_API_CLIENT_ID")
	clientSecret := os.Getenv("YAGPDB_PATREON_API_CLIENT_SECRET")

	if accessToken == "" || clientID == "" || clientSecret == "" {
		PatreonDisabled(nil, "Missing one of YAGPDB_PATREON_API_ACCESS_TOKEN, YAGPDB_PATREON_API_CLIENT_ID, YAGPDB_PATREON_API_CLIENT_SECRET")
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

	// Either use the token provided in the env vars or a cached one in redis
	pClient := patreon.NewClient(tc)
	user, err := pClient.FetchUser()
	if err != nil {
		if storedRefreshToken == "" {
			PatreonDisabled(err, "Failed fetching current user with env var refresh token, no refresh token stored in redis.")
			return
		}

		logrus.WithError(err).Warn("Patreon: Failed fetching current user with env var refresh token, trying stored token")
		tCop := *token
		tCop.RefreshToken = storedRefreshToken

		tc = oauth2.NewClient(context.Background(), &TokenSourceSaver{inner: config.TokenSource(context.Background(), &tCop)})
		pClient = patreon.NewClient(tc)

		user, err = pClient.FetchUser()
		if err != nil {
			PatreonDisabled(err, "Unable to fetch user with redis patreon token.")
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

func PatreonDisabled(err error, reason string) {
	l := logrus.NewEntry(logrus.StandardLogger())

	if err != nil {
		l = l.WithError(err)
	}

	l.Warn("Not starting patreon integration, also means that premium statuses wont update. " + reason)
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

	patrons := make([]*Patron, 0, 25)

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

			user, ok := users[pledge.Relationships.Patron.Data.ID]
			if !ok {
				continue
			}

			patron := &Patron{
				AmountCents: pledge.Attributes.AmountCents,
				Avatar:      user.Attributes.ImageURL,
			}

			if user.Attributes.Vanity != "" {
				patron.Name = user.Attributes.Vanity
			} else {
				patron.Name = user.Attributes.FirstName
			}

			if user.Attributes.SocialConnections.Discord != nil && user.Attributes.SocialConnections.Discord.UserID != "" {
				discordID, _ := strconv.ParseInt(user.Attributes.SocialConnections.Discord.UserID, 10, 64)
				patron.DiscordID = discordID
			}

			patrons = append(patrons, patron)
			// fmt.Printf("%s is pledging %d cents, Discord: %d\r\n", patron.Name, patron.AmountCents, patron.DiscordID)
		}

		// Get the link to the next page of pledges
		nextLink := pledgesResponse.Links.Next
		if nextLink == "" {
			break
		}

		cursor = nextLink
		page++
	}

	patrons = append(patrons, &Patron{
		DiscordID:   common.Conf.Owner,
		Name:        "Owner",
		AmountCents: 10000,
	})

	// Swap the stored ones, this dosent mutate the existing returned slices so we dont have to do any copying on each request woo
	p.mu.Lock()
	p.activePatrons = patrons
	p.mu.Unlock()
}

func (p *Poller) GetPatrons() (patrons []*Patron) {
	p.mu.RLock()
	patrons = p.activePatrons
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
