package common

import (
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

// ExternalHTTPClient is for fetching user-supplied URLs. It refuses to connect to
// non-public addresses, checked after DNS resolution on every connection including redirects.
var ExternalHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: publicOnlyDialControl,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	},
}
