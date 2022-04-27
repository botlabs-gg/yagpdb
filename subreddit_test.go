package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetDefaultSubreddits(t *testing.T) {
	url := fmt.Sprintf("%s/subreddits/default.json", baseURL)
	mockResponseFromFile(url, "test_data/subreddit/default_subreddits.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	subreddits, err := client.GetDefaultSubreddits()
	assert.NoError(t, err)
	assert.Equal(t, len(subreddits), 3, t)
}

func TestGetGoldSubreddits(t *testing.T) {
	url := fmt.Sprintf("%s/subreddits/gold.json", baseURL)
	mockResponseFromFile(url, "test_data/subreddit/gold_subreddits.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	subreddits, err := client.GetGoldSubreddits()
	assert.NoError(t, err)
	assert.Equal(t, len(subreddits), 0)
}

func TestGetNewSubreddits(t *testing.T) {
	url := fmt.Sprintf("%s/subreddits/new.json", baseURL)
	mockResponseFromFile(url, "test_data/subreddit/new_subreddits.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	subreddits, err := client.GetNewSubreddits()
	assert.NoError(t, err)
	assert.Equal(t, len(subreddits), 3)
}

func TestGetPopularSubreddits(t *testing.T) {
	url := fmt.Sprintf("%s/subreddits/popular.json", baseURL)
	mockResponseFromFile(url, "test_data/subreddit/popular_subreddits.json")
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	subreddits, err := client.GetPopularSubreddits()
	assert.NoError(t, err)
	assert.Equal(t, len(subreddits), 3)
}
