package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsUsernameAvailable(t *testing.T) {
	url := fmt.Sprintf("%s/api/username_available.json?user=GovSchwarzenegger", baseURL)
	httpmock.Activate()
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, "false"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	isUsernameAvailable, err := client.IsUsernameAvailable("GovSchwarzenegger")
	assert.NoError(t, err)
	assert.Equal(t, isUsernameAvailable, false)
}

func TestGetUserInfo(t *testing.T) {
	url := fmt.Sprintf("%s/user/GovSchwarzenegger/about.json", baseURL)
	mockResponseFromFile(url, "test_data/user/user_info.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	userInfo, err := client.GetUserInfo("GovSchwarzenegger")
	assert.NoError(t, err)
	assert.Equal(t, userInfo.Name, "GovSchwarzenegger")
}
