package mqueue

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

type RoundTripper struct {
	T         *testing.T
	Expecting string
	Timeout   time.Duration

	ResChan chan bool
}

type FakeReadCloser struct {
}

func (f *FakeReadCloser) Read(b []byte) (n int, err error) {
	b[0] = '{'
	b[1] = '}'
	return 2, io.EOF
}

func (f *FakeReadCloser) Close() error {
	return nil
}

func (r *RoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	defer func() {
		r.ResChan <- true
	}()

	resp = &http.Response{
		StatusCode: 200,
		Body:       &FakeReadCloser{},
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		r.T.Error("Error reading body", err)
		return
	}

	if string(body) != r.Expecting {
		r.T.Error("Unexpected output, got ", string(body), ", expected ", r.Expecting)
		return
	}

	return
}
