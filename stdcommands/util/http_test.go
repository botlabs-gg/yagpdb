package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != DefaultUserAgent || r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(strings.Repeat("a", maxResponseSize+100)))
	}))
	defer srv.Close()

	status, body, err := HTTPGet(context.Background(), srv.URL, http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusTeapot {
		t.Errorf("status = %d, want %d", status, http.StatusTeapot)
	}
	if len(body) != maxResponseSize {
		t.Errorf("body length = %d, want %d", len(body), maxResponseSize)
	}
}

func TestHTTPGetCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := HTTPGet(ctx, srv.URL, nil); err == nil {
		t.Error("expected an error for a cancelled context")
	}
}
