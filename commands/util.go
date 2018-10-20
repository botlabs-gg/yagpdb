package commands

import (
	"errors"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type DurationArg struct {
	Min, Max time.Duration
}

func (d *DurationArg) Matches(def *dcmd.ArgDef, part string) bool {
	if len(part) < 1 {
		return false
	}

	// We "need" the first character to be a number
	r, _ := utf8.DecodeRuneInString(part)
	if !unicode.IsNumber(r) {
		return false
	}

	_, err := ParseDuration(part)
	return err == nil
}

func (d *DurationArg) Parse(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	dur, err := ParseDuration(part)
	if err != nil {
		return nil, err
	}

	if d.Min != 0 && d.Min > dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	if d.Max != 0 && d.Max < dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	return dur, nil
}

func (d *DurationArg) HelpName() string {
	return "Duration"
}

// Parses a time string like 1day3h
func ParseDuration(str string) (time.Duration, error) {
	var dur time.Duration

	currentNumBuf := ""
	currentModifierBuf := ""

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
					return d, err
				}

				dur += d

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
		if err == nil {
			dur += d
		}
	}

	return dur, nil
}

func parseDurationComponent(numStr, modifierStr string) (time.Duration, error) {
	parsedNum, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	parsedDur := time.Duration(parsedNum)

	if strings.HasPrefix(modifierStr, "s") {
		parsedDur = parsedDur * time.Second
	} else if modifierStr == "" || (strings.HasPrefix(modifierStr, "m") && (len(modifierStr) < 2 || modifierStr[1] != 'o')) {
		parsedDur = parsedDur * time.Minute
	} else if strings.HasPrefix(modifierStr, "h") {
		parsedDur = parsedDur * time.Hour
	} else if strings.HasPrefix(modifierStr, "d") {
		parsedDur = parsedDur * time.Hour * 24
	} else if strings.HasPrefix(modifierStr, "w") {
		parsedDur = parsedDur * time.Hour * 24 * 7
	} else if strings.HasPrefix(modifierStr, "mo") {
		parsedDur = parsedDur * time.Hour * 24 * 30
	} else if strings.HasPrefix(modifierStr, "y") {
		parsedDur = parsedDur * time.Hour * 24 * 365
	} else {
		return parsedDur, errors.New("Couldn't figure out what '" + numStr + modifierStr + "` was")
	}

	return parsedDur, nil

}

type DurationOutOfRangeError struct {
	Min, Max time.Duration
	Got      time.Duration
	ArgName  string
}

func (o *DurationOutOfRangeError) Error() string {
	preStr := "too big"
	if o.Got < o.Min {
		preStr = "too small"
	}

	if o.Min == 0 {
		return fmt.Sprintf("%s is %s, has to be smaller than %s", o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Max))
	} else if o.Max == 0 {
		return fmt.Sprintf("%s is %s, has to be bigger than %s", o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Min))
	} else {
		format := "%s is %s (has to be within `%d` and `%d`)"
		return fmt.Sprintf(format, o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Min), common.HumanizeDuration(common.DurationPrecisionMinutes, o.Max))
	}
}

type PublicError string

func (p PublicError) Error() string {
	return string(p)
}

func NewPublicError(a ...interface{}) PublicError {
	return PublicError(fmt.Sprint(a...))
}

func NewPublicErrorF(f string, a ...interface{}) PublicError {
	return PublicError(fmt.Sprintf(f, a...))
}

func FilterBadInvites(msg string, guildID int64, replacement string) string {
	return common.ReplaceServerInvites(msg, guildID, replacement)
}
