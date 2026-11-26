package common

import (
	"io"
	"net/netip"
	"strings"
	"testing"

	"emperror.dev/errors"
)

func TestIsPublicAddrBlocksEmbeddedIPv4(t *testing.T) {
	for _, s := range []string{"::7f00:1", "::a00:1", "2001::1", "2002:7f00:1::", "64:ff9b::7f00:1", "127.0.0.1", "10.0.0.1", "::ffff:127.0.0.1"} {
		if isPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should not be public", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !isPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s should be public", s)
		}
	}
}

func TestLimitedBody(t *testing.T) {
	read := func(body string, max int64) (string, error) {
		b, err := io.ReadAll(&limitedBody{rc: io.NopCloser(strings.NewReader(body)), remaining: max})
		return string(b), err
	}

	if got, err := read("hello", 5); err != nil || got != "hello" {
		t.Fatalf("exact size: got %q, %v", got, err)
	}
	if got, err := read("hi", 5); err != nil || got != "hi" {
		t.Fatalf("under size: got %q, %v", got, err)
	}
	if _, err := read("hello world", 5); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("over size: expected ErrResponseTooLarge, got %v", err)
	}
}
