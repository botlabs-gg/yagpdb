package reddit

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

type Error struct {
	Code    int
	Status  string
	Data    string
	Headers http.Header
}

func (e *Error) Error() string {
	return fmt.Sprintf("Reddit returned HTTP %s (%d) Headers: %v, Response: %s", e.Status, e.Code, e.Headers, e.Data)
}

func NewError(resp *http.Response) error {
	defer resp.Body.Close()

	d, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<20))

	return &Error{
		Code:    resp.StatusCode,
		Status:  resp.Status,
		Data:    string(d),
		Headers: resp.Header,
	}
}
