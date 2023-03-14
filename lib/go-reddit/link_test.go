package reddit

import (
	"fmt"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestDeleteLink(t *testing.T) {
	url := fmt.Sprintf("%s/api/del", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.DeleteLink("5ans3h")
	assert.NoError(t, err)
}

func TestCommentOnLink(t *testing.T) {
	url := fmt.Sprintf("%s/api/comment", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.CommentOnLink("5ans3h", "Hello World!")
	assert.NoError(t, err)
}

func TestEditLinkText(t *testing.T) {
	url := fmt.Sprintf("%s/api/editusertext", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.EditLinkText("5ans3h", "Hello World!")
	assert.NoError(t, err)
}

func TestGetHotLinks(t *testing.T) {
	url := fmt.Sprintf("%s/r/news/hot.json", baseURL)
	mockResponseFromFile(url, "test_data/link/hot_links.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	links, err := client.GetHotLinks("news")
	assert.NoError(t, err)
	assert.Equal(t, len(links), 3)
}

func TestGetNewLinks(t *testing.T) {
	url := fmt.Sprintf("%s/r/news/new.json", baseURL)
	mockResponseFromFile(url, "test_data/link/new_links.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	links, err := client.GetNewLinks("news", "", "")
	assert.NoError(t, err)
	assert.Equal(t, len(links), 3)
}

func TestGetTopLinks(t *testing.T) {
	url := fmt.Sprintf("%s/r/news/top.json", baseURL)
	mockResponseFromFile(url, "test_data/link/top_links.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	links, err := client.GetTopLinks("news")
	assert.NoError(t, err)
	assert.Equal(t, len(links), 3)
}

func TestHideLink(t *testing.T) {
	url := fmt.Sprintf("%s/api/hide", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.HideLink("5ans3h")
	assert.NoError(t, err)
}
