package reddit

import (
	"context"
	"errors"
	"golang.org/x/oauth2"
	"net/http"
)

// Authenticator provides functions for authenticating a user via OAuth2 and generating a client that can be used to access authorized API endpoints.
type Authenticator struct {
	config *oauth2.Config
	state  string
	ua     string
}

const (
	// ScopeIdentity allows access to account information.
	ScopeIdentity = "identity"
	// ScopeEdit allows modification and deletion of comments and submissions.
	ScopeEdit = "edit"
	// ScopeFlair allows modification of user link flair on submissions.
	ScopeFlair = "flair"
	// ScopeHistory allows access to user voting history on comments and submissions
	ScopeHistory = "history"
	// ScopeModConfig allows management of configuration, sidebar, and CSS of user managed subreddits.
	ScopeModConfig = "modconfig"
	// ScopeModFlair allows management and assignment of user moderated subreddits.
	ScopeModFlair = "modflair"
	// ScopeModLog allows access to moderation log for user moderated subreddits.
	ScopeModLog = "modlog"
	// ScopeModWiki allows changing of editors and visibility of wiki pages in user moderated subreddits.
	ScopeModWiki = "modwiki"
	// ScopeMySubreddits allows access to the list of subreddits user moderates, contributes to, and is subscribed to.
	ScopeMySubreddits = "mysubreddits"
	// ScopePrivateMessages allows access to user inbox and the sending of private messages to other users.
	ScopePrivateMessages = "privatemessages"
	// ScopeRead allows access to user posts and comments.
	ScopeRead = "read"
	// ScopeReport allows reporting of content for rules violations.
	ScopeReport = "report"
	// ScopeSave allows saving and unsaving of user comments and submissions.
	ScopeSave = "save"
	// ScopeSubmit allows user submission of links and comments.
	ScopeSubmit = "submit"
	// ScopeSubscribe allows management of user subreddit subscriptions and friends.
	ScopeSubscribe = "subscribe"
	// ScopeVote allows user submission and changing of votes on comments and submissions.
	ScopeVote = "vote"
	// ScopeWikiEdit allows user editing of wiki pages.
	ScopeWikiEdit = "wikiedit"
	// ScopeWikiRead allow user viewing of wiki pages.
	ScopeWikiRead = "wikiread"

	authURL  = "https://www.reddit.com/api/v1/authorize"
	tokenURL = "https://www.reddit.com/api/v1/access_token"
)

// NewAuthenticator generates a new authenticator with the supplied client, state, and requested scopes.
func NewAuthenticator(ua string, clientID string, clientSecret string, redirectURL string, state string, scopes ...string) *Authenticator {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		RedirectURL: redirectURL,
	}

	return &Authenticator{config: config, state: state, ua: ua}
}

// GetAuthenticationURL retrieves the URL used to direct the authenticating user to Reddit for permissions approval.
func (a *Authenticator) GetAuthenticationURL() string {
	return a.config.AuthCodeURL(a.state)
}

// GetToken exchanges an authorization code for an access token.
func (a *Authenticator) GetToken(state string, code string) (*oauth2.Token, error) {
	if state != a.state {
		return nil, errors.New("Invalid state")
	}

	return a.config.Exchange(context.WithValue(oauth2.NoContext, oauth2.HTTPClient, &http.Client{Transport: &uaSetterTransport{agent: a.ua}}), code)
}

// GetAuthClient generates a new authenticated client using the supplied access token.
func (a *Authenticator) GetAuthClient(token *oauth2.Token, userAgent string) *Client {
	return &Client{
		http:      a.httpClient(token),
		userAgent: userAgent,
	}
}

func (a *Authenticator) httpClient(token *oauth2.Token) *http.Client {
	uaSetter := &uaSetterTransport{agent: a.ua}

	tr := oauth2.Transport{
		Source: a.config.TokenSource(context.WithValue(oauth2.NoContext, oauth2.HTTPClient, &http.Client{Transport: uaSetter}), token),
		Base:   &http.Transport{},
	}

	return &http.Client{Transport: &tr}
}

// // agentForward forwards a user agent in all requests made by the Transport.
// type agentForwarder struct {
// 	http.Transport
// 	agent string
// }

// // RoundTrip sets a predefined agent in the request and then forwards it to the
// // default RountTrip implementation.
// func (a *agentForwarder) RoundTrip(r *http.Request) (*http.Response, error) {
// 	r.Header.Add("User-Agent", a.agent)
// 	return a.Transport.RoundTrip(r)
// }

// func clientWithAgent(agent string) *http.Client {
// 	return &http.Client{Transport: &agentForwarder{agent: agent}}
// }

type uaSetterTransport struct {
	t     http.Transport
	agent string
}

func (ua *uaSetterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// modify req here
	req.Header.Add("User-Agent", ua.agent)
	// fmt.Println("REDDIT: roundtripping", req.URL.String(), "UA:", ua.agent)
	return ua.t.RoundTrip(req)
}
