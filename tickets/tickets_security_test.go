package tickets

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/tickets/models"
)

func TestTicketStaffAndAdminChecks(t *testing.T) {
	conf := &models.TicketConfig{ModRoles: []int64{10}, AdminRoles: []int64{20}}

	cases := []struct {
		name          string
		roles         []int64
		perms         int64
		expectedStaff bool
		expectedAdmin bool
	}{
		{"plain member", []int64{1}, discordgo.PermissionSendMessages, false, false},
		{"mod role", []int64{10}, 0, true, false},
		{"admin role", []int64{20}, 0, true, true},
		{"manage channels", nil, discordgo.PermissionManageChannels, true, false},
		{"manage guild", nil, discordgo.PermissionManageGuild, true, true},
		{"administrator", nil, discordgo.PermissionAdministrator, true, true},
		{"manage threads only", nil, discordgo.PermissionManageThreads, false, false},
	}

	for _, c := range cases {
		if got := isTicketStaff(conf, c.roles, c.perms); got != c.expectedStaff {
			t.Errorf("%s: isTicketStaff = %v, expected %v", c.name, got, c.expectedStaff)
		}
		if got := isTicketAdmin(conf, c.roles, c.perms); got != c.expectedAdmin {
			t.Errorf("%s: isTicketAdmin = %v, expected %v", c.name, got, c.expectedAdmin)
		}
	}
}

func TestDownloadAttachment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/small":
			w.Write([]byte("hello"))
		case "/big":
			w.Write([]byte(strings.Repeat("a", 100)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	data, err := downloadAttachment(srv.URL+"/small", 10)
	if err != nil || string(data) != "hello" {
		t.Fatalf("small: got %q, %v", data, err)
	}

	data, err = downloadAttachment(srv.URL+"/small", 5)
	if err != nil || string(data) != "hello" {
		t.Fatalf("exact limit: got %q, %v", data, err)
	}

	if _, err = downloadAttachment(srv.URL+"/big", 50); !errors.Is(err, errAttachmentTooLarge) {
		t.Fatalf("big: expected errAttachmentTooLarge, got %v", err)
	}

	if _, err = downloadAttachment(srv.URL+"/missing", 50); err == nil {
		t.Fatal("missing: expected error")
	}
}
