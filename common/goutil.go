package common

import (
	"slices"
	"strings"
)

// ContainsStringSliceFold reports whether strs contains search case-insensitively.
func ContainsStringSliceFold(strs []string, search string) bool {
	return slices.ContainsFunc(strs, func(str string) bool {
		return strings.EqualFold(str, search)
	})
}

// ContainsInt64SliceOneOf returns true if slice contains one of search
func ContainsInt64SliceOneOf(slice []int64, search []int64) bool {
	return slices.ContainsFunc(slice, func(n int64) bool {
		return slices.Contains(search, n)
	})
}

func IsNumber(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}

	return false
}
