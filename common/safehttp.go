package common

import (
	"io"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"emperror.dev/errors"
)

var ErrBlockedAddress = errors.New("destination address is not allowed")

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2002::/16"),
	// deprecated IPv4-compatible addresses and Teredo, both of which embed an IPv4 address
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("2001::/32"),
}

// MaxExternalResponseBytes caps how much ExternalHTTPClient reads from a response body.
const MaxExternalResponseBytes = 50 << 20

var ErrResponseTooLarge = errors.New("response body is too large")

type limitedBodyTransport struct {
	inner http.RoundTripper
	max   int64
}

func (t *limitedBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	resp.Body = &limitedBody{rc: resp.Body, remaining: t.max}
	return resp, nil
}

// limitedBody fails rather than truncating, so a cut off body is never mistaken for a complete one.
type limitedBody struct {
	rc        io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		var probe [1]byte
		if n, _ := b.rc.Read(probe[:]); n > 0 {
			return 0, ErrResponseTooLarge
		}
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.rc.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error {
	return b.rc.Close()
}

func isPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}

	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}

	return true
}

func publicOnlyDialControl(network, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return err
	}

	if !isPublicAddr(addrPort.Addr()) {
		return ErrBlockedAddress
	}

	return nil
}

// PublicOnlyDialContext refuses to connect to non-public addresses, checked after DNS resolution
// on every connection including redirects.
var PublicOnlyDialContext = (&net.Dialer{
	Timeout: 10 * time.Second,
	Control: publicOnlyDialControl,
}).DialContext

// ExternalHTTPClient is for fetching user-supplied URLs, see PublicOnlyDialContext.
var ExternalHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &limitedBodyTransport{
		max: MaxExternalResponseBytes,
		inner: &http.Transport{
			Proxy:                 nil,
			DialContext:           PublicOnlyDialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
	},
}
