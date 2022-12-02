package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMyTrophies(t *testing.T) {
	url := fmt.Sprintf("%s/api/v1/me/trophies", baseAuthURL)
	mockResponseFromFile(url, "test_data/award/my_trophies.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	trophies, err := client.GetMyTrophies()
	assert.NoError(t, err)
	assert.Equal(t, len(trophies), 1)
}
