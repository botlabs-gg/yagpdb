package patreon

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/patreon/patreonapi"
	"github.com/mediocregopher/radix/v3"
	"golang.org/x/oauth2"
)

var logger = common.GetFixedPrefixLogger("patreon")

type Poller struct {
	mu                   sync.RWMutex
	config               *oauth2.Config
	token                *oauth2.Token
	client               *patreonapi.Client
	lastSuccesfulFetchAt time.Time
	isLastFetchSuccess   bool
	activePatrons        []*Patron
}

var (
	confAccessToken  = config.RegisterOption("yagpdb.patreon.api_access_token", "Access token for the patreon integration", "")
	confRefreshToken = config.RegisterOption("yagpdb.patreon.api_refresh_token", "Refresh token for the patreon integration", "")
	confClientID     = config.RegisterOption("yagpdb.patreon.api_client_id", "Client id for the patreon integration", "")
	confClientSecret = config.RegisterOption("yagpdb.patreon.api_client_secret", "Client secret for the patreon integration", "")
)

func Run() {

	accessToken := confAccessToken.GetString()
	refreshToken := confRefreshToken.GetString()
	clientID := confClientID.GetString()
	clientSecret := confClientSecret.GetString()

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
			AuthURL:  patreonapi.AuthorizationURL,
			TokenURL: patreonapi.AccessTokenURL,
		},
		Scopes: []string{"identity", "campaigns", "campaigns.members"},
	}

	token := &oauth2.Token{
		AccessToken:  "",
		RefreshToken: refreshToken,
		// Must be non-nil, otherwise token will not be expired
		Expiry: time.Now().Add(-24 * time.Hour),
	}

	tc := oauth2.NewClient(context.Background(), &TokenSourceSaver{inner: config.TokenSource(context.Background(), token)})

	// Either use the token provided in the env vars or a cached one in redis
	pClient := patreonapi.NewClient(tc)
	user, err := pClient.FetchUser()
	if err != nil {
		if storedRefreshToken == "" {
			PatreonDisabled(err, "Failed fetching current user with env var refresh token, no refresh token stored in redis.")
			return
		}

		logger.WithError(err).Warn("Failed fetching current user with env var refresh token, trying stored token")
		tCop := *token
		tCop.RefreshToken = storedRefreshToken

		tc = oauth2.NewClient(context.Background(), &TokenSourceSaver{inner: config.TokenSource(context.Background(), &tCop)})
		pClient = patreonapi.NewClient(tc)

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

	logger.Info("Patreon integration activated as ", user.Data.ID, ": ", user.Data.Attributes.FullName)
	go poller.Run()
}

func PatreonDisabled(err error, reason string) {
	l := logger

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

func (p *Poller) IsLastFetchSuccess() bool {
	if p.activePatrons == nil || len(p.activePatrons) < 1 {
		return false
	}
	if time.Since(p.lastSuccesfulFetchAt) < time.Minute*5 {
		return p.isLastFetchSuccess
	}
	return false
}

func (p *Poller) Poll() {
	// Get your campaign data
	logger.Infof("Fetching campaign")
	campaignResponse, err := p.client.FetchCampaigns()
	if err != nil || len(campaignResponse.Data) < 1 {
		logger.WithError(err).Error("Failed fetching campaign")
		p.isLastFetchSuccess = false
		return
	}

	campaignId := campaignResponse.Data[0].ID
	logger.Infof("Successfully fetched campaign: %s ", campaignId)

	cursor := ""
	page := 1

	patrons := make([]*Patron, 0, 30)

	for {
		logger.Infof("Fetching Patrons, page: %d ", page)
		membersResponse, err := p.client.FetchMembers(campaignId, 200, cursor)
		if err != nil {
			logger.WithError(err).Error("Failed fetching pledges")
			p.isLastFetchSuccess = false
			return
		}

		// Get all the users in an easy-to-lookup way
		users := make(map[string]*patreonapi.UserAttributes)
		tiers := make(map[string]*patreonapi.TierAttributes)
		for _, item := range membersResponse.Included {
			if u, ok := item.Decoded.(*patreonapi.UserAttributes); ok {
				users[item.ID] = u
			}
			if t, ok := item.Decoded.(*patreonapi.TierAttributes); ok {
				tiers[item.ID] = t
			}
		}

		// Loop over the pledges to get e.g. their amount and user name
		for _, memberData := range membersResponse.Data {
			attributes := memberData.Attributes
			if memberData.ID == "7997692f-610e-446f-b9c7-ffe198cb7808" {
				logger.Printf("%#v", memberData)
			}
			user, ok := users[memberData.Relationships.User.Data.ID]
			tierCents := 0
			if len(memberData.Relationships.Tiers.Data) > 0 {
				for _, tier := range memberData.Relationships.Tiers.Data {
					if t, ok := tiers[tier.ID]; ok && t.AmountCents > tierCents {
						tierCents = t.AmountCents
					}
				}
			}

			if !ok {
				logger.Infof("Unknown user: %s", memberData.ID)
				continue
			}

			if attributes.PatronStatus != "active_patron" {
				continue
			}

			if len(attributes.LastChargeStatus) > 0 && attributes.LastChargeStatus != patreonapi.ChargeStatusPaid && attributes.LastChargeStatus != patreonapi.ChargeStatusPending {
				continue
			}

			// Skip if the next charge date is in the past
			if attributes.NextChargeDate != nil && attributes.NextChargeDate.Before(time.Now()) {
				continue
			}

			patron := &Patron{
				AmountCents: attributes.CurrentEntitledAmountCents,
				Avatar:      user.ImageURL,
			}

			if patron.AmountCents == 0 {
				patron.AmountCents = tierCents
			}

			if user.Vanity != "" {
				patron.Name = user.Vanity
			} else {
				patron.Name = user.FirstName
			}

			if user.SocialConnections != nil && user.SocialConnections.Discord != nil && user.SocialConnections.Discord.UserID != "" {
				discordID, _ := strconv.ParseInt(user.SocialConnections.Discord.UserID, 10, 64)
				patron.DiscordID = discordID
			}

			patrons = append(patrons, patron)
		}

		// Get the link to the next page of pledges
		nextCursor := membersResponse.Meta.Pagination.Cursors.Next
		if nextCursor == "" {
			break
		}

		cursor = nextCursor
		page++
		time.Sleep(time.Second)
	}
	logger.Infof("Got total %d patrons", len(patrons))
	// Swap the stored ones, this dosent mutate the existing returned slices so we dont have to do any copying on each request woo
	p.mu.Lock()
	p.activePatrons = patrons
	p.lastSuccesfulFetchAt = time.Now()
	p.isLastFetchSuccess = true
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
			logger.Info("New refresh token")
			common.RedisPool.Do(radix.Cmd(nil, "SET", "patreon_refresh_token", tk.RefreshToken))
			t.lastRefreshToken = tk.RefreshToken
		}
	}

	return tk, err
}
