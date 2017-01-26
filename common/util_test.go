package common

import (
	"errors"
	"testing"
)

func TestErrWithCaller(t *testing.T) {
	innerError := errors.New("Test Error")
	wrapped := ErrWithCaller(innerError)

	if wrapped.Error() != "common.TestErrWithCaller: Test Error" {
		t.Error("Unexpected output", wrapped.Error())
	}
}
