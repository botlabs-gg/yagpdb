package reddit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAuthenticationURL(t *testing.T) {
	authenticator := NewAuthenticator("go-test", "client_id", "client_secret", "http://localhost:8000", "123456789abcdef", "identity")
	authenticationURL := authenticator.GetAuthenticationURL()

	assert.Equal(t, authenticationURL, "https://www.reddit.com/api/v1/authorize?client_id=client_id&redirect_uri=http%3A%2F%2Flocalhost%3A8000&response_type=code&scope=identity&state=123456789abcdef")
}
