package util

import (
	"context"
	"io"
	"net/http"
	"time"
)

const (
	DefaultUserAgent = "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)"
	maxResponseSize  = 1 << 20
)

// Third party apis can stall without closing the connection, so requests must not pin goroutines.
var HTTPClient = &http.Client{Timeout: 10 * time.Second}

// HTTPGet performs a GET request and returns the status code and at most 1MiB of the body.
func HTTPGet(ctx context.Context, url string, header http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}

	for k, v := range header {
		req.Header[k] = v
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DefaultUserAgent)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}
