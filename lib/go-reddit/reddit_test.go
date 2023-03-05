package reddit

import (
	"github.com/jarcoal/httpmock"
	"io/ioutil"
)

func mockResponseFromFile(url string, filepath string) {
	httpmock.Activate()
	response, _ := ioutil.ReadFile(filepath)
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, string(response)))
}
