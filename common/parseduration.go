package common

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"emperror.dev/errors"
)

var ErrDurationTooLong = errors.New("duration is too long")

func addDuration(a, b time.Duration) (time.Duration, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, ErrDurationTooLong
	}
	return a + b, nil
}

// Parses a time string like 1day3h
func ParseDuration(str string) (time.Duration, error) {
	var dur time.Duration
	var currentNumBuf, currentModifierBuf string

	// Parse the time
	for _, v := range str {
		// Ignore whitespace
		if unicode.Is(unicode.White_Space, v) {
			continue
		}

		if unicode.IsNumber(v) {
			// If we reached a number and the modifier was also set, parse the last duration component before starting a new one
			if currentModifierBuf != "" {
				if currentNumBuf == "" {
					currentNumBuf = "1"
				}
				d, err := parseDurationComponent(currentNumBuf, currentModifierBuf)
				if err != nil {
					return 0, err
				}

				if dur, err = addDuration(dur, d); err != nil {
					return 0, err
				}

				currentNumBuf = ""
				currentModifierBuf = ""
			}

			currentNumBuf += string(v)

		} else {
			currentModifierBuf += string(v)
		}
	}

	if currentNumBuf != "" {
		d, err := parseDurationComponent(currentNumBuf, currentModifierBuf)
		if err != nil {
			return 0, errors.WrapIf(err, "not a duration")
		}

		if dur, err = addDuration(dur, d); err != nil {
			return 0, err
		}
	}

	return dur, nil
}

func parseDurationComponent(numStr, modifierStr string) (time.Duration, error) {
	parsedNum, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	var unit time.Duration
	switch {
	case strings.HasPrefix(modifierStr, "s"):
		unit = time.Second
	case modifierStr == "", (strings.HasPrefix(modifierStr, "m") && (len(modifierStr) < 2 || modifierStr[1] != 'o')):
		unit = time.Minute
	case strings.HasPrefix(modifierStr, "h"):
		unit = time.Hour
	case strings.HasPrefix(modifierStr, "d"):
		unit = time.Hour * 24
	case strings.HasPrefix(modifierStr, "w"):
		unit = time.Hour * 24 * 7
	case strings.HasPrefix(modifierStr, "mo"):
		unit = time.Hour * 24 * 30
	case strings.HasPrefix(modifierStr, "y"):
		unit = time.Hour * 24 * 365
	default:
		return 0, errors.New("couldn't figure out what '" + numStr + modifierStr + "' was")
	}

	if parsedNum < 0 || parsedNum > math.MaxInt64/int64(unit) {
		return 0, ErrDurationTooLong
	}

	parsedDur := time.Duration(parsedNum) * unit

	return parsedDur, nil

}
