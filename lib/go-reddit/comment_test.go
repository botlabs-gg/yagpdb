package reddit

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDeleteComment(t *testing.T) {
	url := fmt.Sprintf("%s/api/del", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.DeleteComment("d9hthja")
	assert.NoError(t, err)
}

func TestEditCommentText(t *testing.T) {
	url := fmt.Sprintf("%s/api/editusertext", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.EditCommentText("d9hthja", "Hello World!")
	assert.NoError(t, err)
}

func TestReplyToComment(t *testing.T) {
	url := fmt.Sprintf("%s/api/comment", baseAuthURL)
	httpmock.Activate()
	httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(200, "{}"))
	defer httpmock.DeactivateAndReset()

	client := NoAuthClient
	err := client.ReplyToComment("d9hthja", "Hello World!")
	assert.NoError(t, err)
}
