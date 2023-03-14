package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestGetMyPreferences(t *testing.T) {
	url := fmt.Sprintf("%s/api/v1/me/preferences", baseAuthURL)
	mockResponseFromFile(url, "test_data/preferences/my_preferences.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	preferences, err := client.GetMyPreferences()
	assert.NoError(t, err)
	assert.Equal(t, preferences.NumComments, 200)
	assert.Equal(t, preferences.Lang, "en")
	assert.Equal(t, preferences.ShowFlair, true)
}

func TestUpdateMyPreferences(t *testing.T) {
	url := fmt.Sprintf("%s/api/v1/me/preferences", baseAuthURL)
	httpmock.Activate()
	response, _ := ioutil.ReadFile("test_data/preferences/my_preferences.json")
	httpmock.RegisterResponder("PATCH", url, httpmock.NewStringResponder(200, string(response)))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	preferences := &Preferences{NumComments: 600}
	_, err := client.UpdateMyPreferences(preferences)
	assert.NoError(t, err)
}
