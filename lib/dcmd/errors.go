package dcmd

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

type InvalidInt struct {
	Part string
}

func (i *InvalidInt) Error() string {
	return fmt.Sprintf("%q is not a whole number", i.Part)
}

func (i *InvalidInt) IsUserError() bool {
	return true
}

type ErrResolvedNotFound struct {
	Key  string
	ID   int64
	Type string
}

func (r *ErrResolvedNotFound) Error() string {
	return fmt.Sprintf("could not find resolved %s for arg %q with id %d", r.Type, r.Key, r.ID)
}

func (r *ErrResolvedNotFound) IsUserError() bool {
	return true
}

type ErrArgExpectedType struct {
	Name     string
	Expected string
	Got      string
}

func NewErrArgExpected(name string, expected string, got interface{}) error {
	gotStr := ""
	if got == nil {
		gotStr = "nil"
	} else {
		gotStr = reflect.TypeOf(got).String()
	}

	return &ErrArgExpectedType{
		Name:     name,
		Expected: expected,
		Got:      gotStr,
	}
}

func (e *ErrArgExpectedType) Error() string {
	return fmt.Sprintf("%s: got wrong argument type, expected: %q got %q", e.Name, e.Expected, e.Got)
}

func (e *ErrArgExpectedType) IsUserError() bool {
	return true
}

type InvalidFloat struct {
	Part string
}

func (i *InvalidFloat) Error() string {
	return fmt.Sprintf("%q is not a number", i.Part)
}

func (i *InvalidFloat) IsUserError() bool {
	return true
}

type ImproperMention struct {
	Part string
}

func (i *ImproperMention) Error() string {
	return fmt.Sprintf("Improper mention %q", i.Part)
}

func (i *ImproperMention) IsUserError() bool {
	return true
}

type NoMention struct {
	Part string
}

func (i *NoMention) Error() string {
	return fmt.Sprintf("No mention found in %q", i.Part)
}

func (i *NoMention) IsUserError() bool {
	return true
}

type UserNotFound struct {
	Part string
}

func (i *UserNotFound) Error() string {
	return fmt.Sprintf("User %q not found", i.Part)
}

func (i *UserNotFound) IsUserError() bool {
	return true
}

type ChannelNotFound struct {
	ID int64
}

func (c *ChannelNotFound) Error() string {
	return fmt.Sprintf("Channel %d not found", c.ID)
}

func (c *ChannelNotFound) IsUserError() bool {
	return true
}

type OutOfRangeError struct {
	Min, Max interface{}
	Got      interface{}
	Float    bool
	ArgName  string
}

func (o *OutOfRangeError) Error() string {
	preStr := "too big"

	switch o.Got.(type) {
	case int64:
		if o.Got.(int64) < o.Min.(int64) {
			preStr = "too small"
		}
	case float64:
		if o.Got.(float64) < o.Min.(float64) {
			preStr = "too small"
		}
	}

	const floatFormat = "%s is %s (has to be within %f - %f)"
	const intFormat = "%s is %s (has to be within %d - %d)"

	if o.Float {
		return fmt.Sprintf(floatFormat, o.ArgName, preStr, o.Min, o.Max)
	}

	return fmt.Sprintf(intFormat, o.ArgName, preStr, o.Min, o.Max)
}

func (o *OutOfRangeError) IsUserError() bool {
	return true
}

type UserError interface {
	IsUserError() bool
}

func IsUserError(err error) bool {
	v, ok := errors.Cause(err).(UserError)
	if ok && v.IsUserError() {
		return true
	}

	return false
}

type simpleUserError string

func (s simpleUserError) Error() string {
	return string(s)
}

func (s simpleUserError) IsUserError() bool {
	return true
}

func NewSimpleUserError(args ...interface{}) error {
	return simpleUserError(fmt.Sprint(args...))
}
