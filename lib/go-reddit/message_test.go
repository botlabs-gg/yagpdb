package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplyToMessage(t *testing.T) {
	url := fmt.Sprintf("%s/api/comment", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.ReplyToMessage("6qys1q", "Reply")
	assert.NoError(t, err)
}
