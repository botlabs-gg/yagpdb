package rss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/mmcdole/gofeed"
)

// feedState is what we remember about a feed url between polls: the validators needed
// for conditional requests, and how it has been behaving
type feedState struct {
	ETag         string
	LastModified string

	LastSuccess   time.Time
	FirstFailure  time.Time
	LastError     string
	CooldownUntil time.Time
}

type fetchResult struct {
	Feed        *gofeed.Feed
	NotModified bool

	ETag         string
	LastModified string
}

type fetchError struct {
	StatusCode int
	RetryAfter time.Duration
	Err        error
}

func (e *fetchError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("feed host returned %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
	return e.Err.Error()
}

func (e *fetchError) Unwrap() error {
	return e.Err
}

var (
	feedClientOnce sync.Once
	feedClient     *http.Client
)

// feedHTTPClient is shared across fetches so connections are pooled and reused
func feedHTTPClient() *http.Client {
	feedClientOnce.Do(func() {
		transport := http.DefaultTransport.(*http.Transport).Clone()

		if proxy := common.ConfHttpProxy.GetString(); len(proxy) > 0 {
			proxyURL, err := url.Parse(proxy)
			if err != nil {
				logger.WithError(err).WithField("proxy", proxy).Warn("Invalid HTTP proxy configured for RSS fetcher, falling back to direct fetch")
			} else {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}

		feedClient = &http.Client{
			Timeout:   feedFetchTimeout,
			Transport: transport,
		}
	})

	return feedClient
}

// fetchFeed fetches and parses a feed url. when state carries validators from an earlier
// fetch they're sent along, and a 304 comes back as a result with NotModified set.
// every error it returns is a *fetchError.
func fetchFeed(ctx context.Context, feedURL string, state *feedState) (*fetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, &fetchError{Err: err}
	}

	req.Header.Set("User-Agent", feedUserAgent)
	req.Header.Set("Accept", feedAccept)
	if state != nil {
		if state.ETag != "" {
			req.Header.Set("If-None-Match", state.ETag)
		}
		if state.LastModified != "" {
			req.Header.Set("If-Modified-Since", state.LastModified)
		}
	}

	resp, err := feedHTTPClient().Do(req)
	if err != nil {
		return nil, &fetchError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		drainBody(resp)
		return &fetchResult{NotModified: true}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		drainBody(resp)
		return nil, &fetchError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	// a feed that doesn't fit in memory is a feed we don't post
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedSize+1))
	if err != nil {
		return nil, &fetchError{Err: err}
	}
	if len(body) > maxFeedSize {
		return nil, &fetchError{Err: fmt.Errorf("feed is bigger than the %dMB limit", maxFeedSize>>20)}
	}

	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, &fetchError{Err: err}
	}

	return &fetchResult{
		Feed:         feed,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}, nil
}

// drainBody reads the leftovers of a response we're not parsing, so the connection
// can go back in the pool instead of being torn down
func drainBody(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
}

// parseRetryAfter reads a Retry-After header in either of its two forms
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	if at, err := http.ParseTime(header); err == nil {
		if until := time.Until(at); until > 0 {
			return until
		}
	}

	return 0
}

func feedStateKey(feedURL string) string {
	return "rss:feed_state:" + md5Hash(feedURL)
}

// LoadFeedState always returns usable state, even when redis is unavailable
func LoadFeedState(feedURL string) (*feedState, error) {
	var raw map[string]string
	if err := common.RedisPool.Do(radix.Cmd(&raw, "HGETALL", feedStateKey(feedURL))); err != nil {
		return &feedState{}, err
	}

	return &feedState{
		ETag:          raw["etag"],
		LastModified:  raw["last_modified"],
		LastError:     raw["last_error"],
		LastSuccess:   parseUnix(raw["last_success"]),
		FirstFailure:  parseUnix(raw["first_failure"]),
		CooldownUntil: parseUnix(raw["cooldown_until"]),
	}, nil
}

func saveFeedState(feedURL string, set map[string]string, unset ...string) {
	key := feedStateKey(feedURL)

	actions := make([]radix.CmdAction, 0, 3)
	if len(set) > 0 {
		args := make([]string, 0, len(set)*2+1)
		args = append(args, key)
		for field, value := range set {
			args = append(args, field, value)
		}
		actions = append(actions, radix.Cmd(nil, "HSET", args...))
	}
	if len(unset) > 0 {
		actions = append(actions, radix.Cmd(nil, "HDEL", append([]string{key}, unset...)...))
	}
	actions = append(actions, radix.FlatCmd(nil, "EXPIRE", key, int(feedStateTTL.Seconds())))

	if err := common.RedisPool.Do(radix.Pipeline(actions...)); err != nil {
		logger.WithError(err).WithField("url", feedURL).Warn("Failed saving RSS feed state")
	}
}

// recordFetchSuccess stores the validators for the next conditional request and clears
// the failure bookkeeping, giving the feed a fresh grace period if it breaks again
func recordFetchSuccess(feedURL string, res *fetchResult) {
	set := map[string]string{"last_success": formatUnix(time.Now())}
	if !res.NotModified {
		// only a full response carries validators, a 304 keeps the ones we already have
		set["etag"] = res.ETag
		set["last_modified"] = res.LastModified
	}

	saveFeedState(feedURL, set, "first_failure", "last_error", "cooldown_until")
}

func parseUnix(v string) time.Time {
	if v == "" {
		return time.Time{}
	}

	sec, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

func formatUnix(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}
