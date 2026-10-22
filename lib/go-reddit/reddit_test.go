package reddit

import (
	"os"

	"github.com/jarcoal/httpmock"
)

func mockResponseFromFile(url string, filepath string) {
	httpmock.Activate()
	response, _ := os.ReadFile(filepath)
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, string(response)))
}
