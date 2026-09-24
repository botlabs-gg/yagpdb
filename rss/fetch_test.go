package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

const testFeedBody = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
	<title>Test feed</title>
	<item><title>One</title><link>https://example.com/one</link></item>
</channel></rss>`

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("120"); got != 2*time.Minute {
		t.Errorf("seconds form: got %s, want 2m", got)
	}

	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("empty header: got %s, want 0", got)
	}

	if got := parseRetryAfter("soon"); got != 0 {
		t.Errorf("unparseable header: got %s, want 0", got)
	}

	if got := parseRetryAfter("-5"); got != 0 {
		t.Errorf("negative seconds: got %s, want 0", got)
	}

	if got := parseRetryAfter(time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)); got != 0 {
		t.Errorf("date in the past: got %s, want 0", got)
	}

	got := parseRetryAfter(time.Now().Add(30 * time.Minute).UTC().Format(http.TimeFormat))
	if got < 29*time.Minute || got > 30*time.Minute {
		t.Errorf("date form: got %s, want about 30m", got)
	}
}

func TestCooldownFor(t *testing.T) {
	tests := []struct {
		name string
		err  *fetchError
		want time.Duration
	}{
		{"no error", nil, 0},
		{"rate limited without a hint", &fetchError{StatusCode: http.StatusTooManyRequests}, defaultFeedCooldown},
		{"rate limited with a hint", &fetchError{StatusCode: http.StatusTooManyRequests, RetryAfter: time.Minute}, time.Minute},
		{"unavailable without a hint", &fetchError{StatusCode: http.StatusServiceUnavailable}, 0},
		{"unavailable with a hint", &fetchError{StatusCode: http.StatusServiceUnavailable, RetryAfter: time.Hour}, time.Hour},
		{"capped", &fetchError{StatusCode: http.StatusTooManyRequests, RetryAfter: 30 * 24 * time.Hour}, maxFeedCooldown},
		{"not found", &fetchError{StatusCode: http.StatusNotFound}, 0},
		{"request timeout", &fetchError{StatusCode: http.StatusRequestTimeout}, 0},
		{"too early", &fetchError{StatusCode: http.StatusTooEarly}, 0},
		{"network error", &fetchError{Err: context.DeadlineExceeded}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cooldownFor(tc.err); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestIsPermanentStatus(t *testing.T) {
	permanent := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone, http.StatusTeapot, http.StatusUnavailableForLegalReasons}
	for _, status := range permanent {
		if !isPermanentStatus(status) {
			t.Errorf("%d should disable the feed", status)
		}
	}

	transient := []int{http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 0}
	for _, status := range transient {
		if isPermanentStatus(status) {
			t.Errorf("%d should not disable the feed", status)
		}
	}
}

// a rate limited feed must always take the cooldown path, never the disable path
func TestRateLimitedFeedIsNeverPermanent(t *testing.T) {
	for _, err := range []*fetchError{
		{StatusCode: http.StatusTooManyRequests},
		{StatusCode: http.StatusTooManyRequests, RetryAfter: time.Hour},
	} {
		if isPermanentStatus(err.StatusCode) {
			t.Fatal("429 was treated as permanent")
		}
		if cooldownFor(err) <= 0 {
			t.Fatal("429 did not produce a cooldown")
		}
	}
}

func TestFetchFeedConditionalRequest(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		if r.Header.Get("User-Agent") != feedUserAgent {
			t.Errorf("unexpected user agent %q", r.Header.Get("User-Agent"))
		}

		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.Write([]byte(testFeedBody))
	}))
	defer server.Close()

	res, err := fetchFeed(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if res.NotModified {
		t.Fatal("first fetch reported not modified")
	}
	if res.ETag != `"v1"` || res.LastModified != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("validators not picked up: etag %q, last-modified %q", res.ETag, res.LastModified)
	}
	if len(res.Feed.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(res.Feed.Items))
	}

	res, err = fetchFeed(context.Background(), server.URL, &feedState{ETag: `"v1"`})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !res.NotModified {
		t.Fatal("second fetch did not report not modified")
	}
	if requests != 2 {
		t.Fatalf("server saw %d requests, want 2", requests)
	}
}

func TestFetchFeedErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := fetchFeed(context.Background(), server.URL, nil)
	if err == nil {
		t.Fatal("expected an error")
	}

	fErr, ok := err.(*fetchError)
	if !ok {
		t.Fatalf("got %T, want *fetchError", err)
	}
	if fErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("got status %d, want 429", fErr.StatusCode)
	}
	if fErr.RetryAfter != time.Minute {
		t.Errorf("got retry after %s, want 1m", fErr.RetryAfter)
	}
}

func TestFetchFeedRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>`))
		w.Write([]byte(strings.Repeat("a", maxFeedSize)))
	}))
	defer server.Close()

	if _, err := fetchFeed(context.Background(), server.URL, nil); err == nil {
		t.Fatal("expected an error for an oversized feed")
	}
}

func TestFetchFeedRejectsNonFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>not a feed</body></html>"))
	}))
	defer server.Close()

	if _, err := fetchFeed(context.Background(), server.URL, nil); err == nil {
		t.Fatal("expected an error for a non-feed response")
	}
}

func TestCollectPostableItems(t *testing.T) {
	old := time.Now().Add(-maxItemAge - time.Hour)
	recent := time.Now().Add(-time.Hour)

	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{Title: "newest", Link: "https://example.com/3", PublishedParsed: &recent},
		{Title: "no link"},
		{Title: "javascript link", Link: "javascript:alert(1)"},
		{Title: "undated", Link: "https://example.com/2"},
		{Title: "stale", Link: "https://example.com/1", PublishedParsed: &old},
		{Title: "updated only", Link: "https://example.com/0", UpdatedParsed: &recent},
	}}

	items := collectPostableItems(feed)

	var titles []string
	for _, item := range items {
		titles = append(titles, item.Title)
	}

	want := []string{"updated only", "undated", "newest"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", titles, want)
	}

	if items[0].PublishedAt == nil || !items[0].PublishedAt.Equal(recent) {
		t.Error("UpdatedParsed was not used as the publish time")
	}
	if items[1].PublishedAt != nil {
		t.Error("an undated item should have no publish time")
	}
}
