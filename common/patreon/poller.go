package patreon

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/common/patreon/patreonapi"
	"github.com/mediocregopher/radix/v3"
	"golang.org/x/oauth2"
)

var logger = common.GetFixedPrefixLogger("patreon")

type Poller struct {
	mu     sync.RWMutex
	config *oauth2.Config
	token  *oauth2.Token
	client *patreonapi.Client

	activePatrons []*Patron
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

func (p *Poller) Poll() {
	// Get your campaign data
	campaignResponse, err := p.client.FetchCampaigns()
	if err != nil || len(campaignResponse.Data) < 1 {
		logger.WithError(err).Error("Failed fetching campaign")
		return
	}

	campaignId := campaignResponse.Data[0].ID

	cursor := ""
	page := 1

	patrons := make([]*Patron, 0, 30)

	for {
		membersResponse, err := p.client.FetchMembers(campaignId, 0, cursor)
		// pledgesResponse, err := p.client.FetchPledges(campaignId,
		// patreon.WithPageSize(30),
		// patreon.WithCursor(cursor))

		if err != nil {
			logger.WithError(err).Error("Failed fetching pledges")
			return
		}

		// logger.Println("num results: ", len(membersResponse.Data))

		// Get all the users in an easy-to-lookup way
		users := make(map[string]*patreonapi.UserAttributes)
		for _, item := range membersResponse.Included {
			if u, ok := item.Decoded.(*patreonapi.UserAttributes); ok {
				users[item.ID] = u
			}
		}

		// Loop over the pledges to get e.g. their amount and user name
		for _, memberData := range membersResponse.Data {
			attributes := memberData.Attributes

			user, ok := users[memberData.Relationships.User.Data.ID]
			if !ok {
				// logger.Println("Unknown user: ", memberData.ID)
				continue
			}

			if attributes.LastChargeStatus != patreonapi.ChargeStatusPaid && attributes.LastChargeStatus != patreonapi.ChargeStatusPending {
				// logger.Println("Not paid: ", attributes.FullName)
				continue
			}

			if attributes.PatronStatus != "active_patron" {
				continue
			}

			// logger.Println(attributes.PatronStatus + " --- " + user.FirstName + ":" + user.LastName + ":" + user.Vanity)

			patron := &Patron{
				AmountCents: attributes.CurrentEntitledAmountCents,
				Avatar:      user.ImageURL,
			}

			if user.Vanity != "" {
				patron.Name = user.Vanity
			} else {
				patron.Name = user.FirstName
			}

			if user.SocialConnections.Discord != nil && user.SocialConnections.Discord.UserID != "" {
				discordID, _ := strconv.ParseInt(user.SocialConnections.Discord.UserID, 10, 64)
				patron.DiscordID = discordID
			}

			patrons = append(patrons, patron)
			// logger.Printf("%s is pledging %d cents, Discord: %d\r\n", patron.Name, patron.AmountCents, patron.DiscordID)
		}

		// Get the link to the next page of pledges
		nextCursor := membersResponse.Meta.Pagination.Cursors.Next
		if nextCursor == "" {
			// logger.Println("No nextlink ", page)
			break
		}

		cursor = nextCursor
		// logger.Println("nextlink: ", page, ": ", cursor)
		page++
	}

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
			logger.Info("New refresh token")
			common.RedisPool.Do(radix.Cmd(nil, "SET", "patreon_refresh_token", tk.RefreshToken))
			t.lastRefreshToken = tk.RefreshToken
		}
	}

	return tk, err
}
