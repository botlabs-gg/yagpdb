package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func TestSendAddedServerToSetup(t *testing.T) {
	served := false
	handler := SendAddedServerToSetup(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served = true }))

	cases := []struct {
		name     string
		query    string
		loggedIn bool
		want     string
	}{
		{"logged in", "?guild_id=123&code=x", true, "/manage/123/setup?from=invite"},
		{"logged out", "?guild_id=123", false, "/login?goto=%2Fmanage%2F123%2Fsetup%3Ffrom%3Dinvite"},
	}

	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/manage"+c.query, nil)
		if c.loggedIn {
			r = r.WithContext(context.WithValue(r.Context(), common.ContextKeyUser, &discordgo.User{ID: 1}))
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if got := w.Header().Get("Location"); got != c.want {
			t.Errorf("%s: redirected to %q, want %q", c.name, got, c.want)
		}
	}

	served = false
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/manage", nil))
	if !served {
		t.Error("server list was not shown without a guild id")
	}
}
