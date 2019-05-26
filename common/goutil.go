package common

import (
	"strings"
)

func ContainsStringSlice(strs []string, search string) bool {
	for _, v := range strs {
		if v == search {
			return true
		}
	}

	return false
}

func ContainsStringSliceFold(strs []string, search string) bool {
	for _, v := range strs {
		if strings.EqualFold(v, search) {
			return true
		}
	}

	return false
}

func ContainsInt64Slice(slice []int64, search int64) bool {
	for _, v := range slice {
		if v == search {
			return true
		}
	}

	return false
}

// ContainsInt64SliceOneOf returns true if slice contains one of search
func ContainsInt64SliceOneOf(slice []int64, search []int64) bool {
	for _, v := range search {
		if ContainsInt64Slice(slice, v) {
			return true
		}
	}

	return false
}

func ContainsIntSlice(slice []int, search int) bool {
	for _, v := range slice {
		if v == search {
			return true
		}
	}

	return false
}

func IsNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}

	return false
}
