package templates

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func buildLongStr(length int) string {
	var b strings.Builder

	for i := 0; i < length; i++ {
		b.WriteString("A")
	}

	return b.String()
}

func TestJoinStrings(t *testing.T) {

	longString := buildLongStr(1000000)

	cases := []struct {
		sep            string
		args           []interface{}
		expectedResult string
		shouldError    bool
	}{
		{" ", []interface{}{"hello", "world"}, "hello world", false},
		{" ", []interface{}{"hello", "world", longString}, "", true},
		{",", []interface{}{"hello", []string{"world", "!"}}, "hello,world,!", false},
		{" ", []interface{}{[]string{"hello", "world", "!"}}, "hello world !", false},
		{" ", []interface{}{[]string{"hello", "world", "!", longString}}, "", true},
	}

	for i, c := range cases {
		t.Run("case #"+strconv.Itoa(i), func(t *testing.T) {
			joined, err := joinStrings(c.sep, c.args...)
			if err != nil && !c.shouldError {
				t.Errorf("Should not have errored out")
			} else if err == nil && c.shouldError {
				t.Errorf("Should have errored out")
			}
			if joined != c.expectedResult {
				t.Error("Unexpected result, got ", joined, ", expected ", c.expectedResult)
			}
		})
	}
}

func TestSlice(t *testing.T) {
	cases := []struct {
		slice         []interface{}
		start, end    int
		expectedSlice []interface{}
	}{
		{[]interface{}{"a", "b", "c"}, 1, -1, []interface{}{"b", "c"}},
		{[]interface{}{"a", "b", "c"}, 0, 2, []interface{}{"a", "b"}},
	}

	for i, c := range cases {
		t.Run("case #"+strconv.Itoa(i), func(t *testing.T) {
			sliceV := reflect.ValueOf(c.slice)
			args := make([]reflect.Value, 0, 2)
			args = append(args, reflect.ValueOf(c.start))

			if c.end != -1 {
				args = append(args, reflect.ValueOf(c.end))
			}

			result, err := slice(sliceV, args...)
			if err != nil {
				t.Errorf("Got error: %s", err)
			}
			cast := result.Interface().([]interface{})
			if len(cast) != len(c.expectedSlice) {
				t.Errorf("Unexpected result, differing lengths, got: %v, expected: %v", cast, c.expectedSlice)
			}

			for i, v := range cast {
				if c.expectedSlice[i] != v {
					t.Errorf("Unexpected result, differing elements, got: %v, expected: %v", v, c.expectedSlice[i])
				}
			}
		})
	}
}
