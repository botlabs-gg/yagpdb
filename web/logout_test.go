package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogoutReturnsThroughLogin(t *testing.T) {
	cases := map[string]string{
		"/logout":                            "/",
		"/logout?goto=%2F123%2Fappeal":       "/login?goto=%2F123%2Fappeal",
		"/logout?goto=https%3A%2F%2Fevil.io": "/",
		"/logout?goto=%2F%2Fevil.io":         "/",
		"/logout?goto=%2F%5Cevil.io":         "/",
	}

	for target, want := range cases {
		w := httptest.NewRecorder()
		HandleLogout(w, httptest.NewRequest(http.MethodGet, target, nil))
		if got := w.Header().Get("Location"); got != want {
			t.Errorf("%s: redirected to %q, want %q", target, got, want)
		}
	}
}
