package templates

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"

	"emperror.dev/errors"
)

var ErrValueTooLarge = errors.NewPlain("Value is too large (max 1MB)")

type sizeMode int

const (
	// fmt prints nested pointers as addresses and uses String() where present.
	sizeModeFormat sizeMode = iota
	// json and msgpack follow every pointer and skip unexported fields.
	sizeModeEncode
)

const maxSizeWalkDepth = 1000

// exceedsSerializedSize reports whether formatting or encoding values would produce more than limit bytes.
// The estimate is a lower bound: string bytes plus one byte per value visited. A value referenced many times
// is counted every time, which is what catches nesting the same slice into itself repeatedly, and because
// every visit costs at least one byte the walk gives up after at most limit steps.
func exceedsSerializedSize(mode sizeMode, limit int, values ...any) bool {
	remaining := limit

	var walk func(v reflect.Value, depth int) bool
	walk = func(v reflect.Value, depth int) bool {
		remaining--
		if remaining < 0 || depth > maxSizeWalkDepth {
			return true
		}

		if !v.IsValid() {
			return false
		}

		if stopsAtMarshaler(mode, v) {
			return false
		}

		switch v.Kind() {
		case reflect.Interface:
			if v.IsNil() {
				return false
			}
			return walk(v.Elem(), depth+1)

		case reflect.Pointer:
			if v.IsNil() || (mode == sizeModeFormat && depth > 0) {
				return false
			}
			return walk(v.Elem(), depth+1)

		case reflect.String:
			remaining -= v.Len()

		case reflect.Slice, reflect.Array:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				remaining -= v.Len()
				break
			}
			for i := 0; i < v.Len(); i++ {
				if walk(v.Index(i), depth+1) {
					return true
				}
			}

		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				if walk(iter.Key(), depth+1) || walk(iter.Value(), depth+1) {
					return true
				}
			}

		case reflect.Struct:
			t := v.Type()
			for i := 0; i < v.NumField(); i++ {
				if mode == sizeModeEncode && !t.Field(i).IsExported() {
					continue
				}
				if walk(v.Field(i), depth+1) {
					return true
				}
			}
		}

		return remaining < 0
	}

	for _, v := range values {
		if walk(reflect.ValueOf(v), 0) {
			return true
		}
	}
	return false
}

// ExceedsEncodedSize reports whether json or msgpack encoding v would produce more than limit bytes.
func ExceedsEncodedSize(v any, limit int) bool {
	return exceedsSerializedSize(sizeModeEncode, limit, v)
}

var (
	stringerType      = reflect.TypeFor[fmt.Stringer]()
	errorType         = reflect.TypeFor[error]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// Structs with custom output methods produce whatever those methods return, so their fields say nothing
// about the output size. Containers are always walked since that is where template values are nested.
func stopsAtMarshaler(mode sizeMode, v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Struct, reflect.Pointer:
	default:
		return false
	}

	t := v.Type()
	if mode == sizeModeFormat {
		return t.Implements(stringerType) || t.Implements(errorType)
	}
	return t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType)
}
