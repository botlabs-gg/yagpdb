package naturaltime

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/lib/when"
)

//go:embed timezones.json
var timezonesJSON []byte

// tzOffsets maps timezone abbreviations (uppercased) to their UTC offset in minutes.
var tzOffsets map[string]int

// defaultLocation is used when no timezone is found in the input string.
var defaultLocation *time.Location

// utcOffsetPattern matches explicit UTC/GMT offset expressions.
// e.g. "UTC+1", "GMT-5", "UTC+5:30", "gmt-3:30"
var utcOffsetPattern = regexp.MustCompile(`(?i)\b(UTC|GMT)([+-])(\d{1,2})(?::(\d{2}))?\b`)

var logger = common.GetPluginLogger(&Plugin{})

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Natural Time Parser",
		SysName:  "naturaltime",
		Category: common.PluginCategoryMisc,
	}
}

// RegisterPlugin registers naturaltime with YAGPDB.
// Called from cmd/yagpdb/main.go alongside other plugin registrations.
func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

func init() {
	if err := json.Unmarshal(timezonesJSON, &tzOffsets); err != nil {
		logger.WithError(err).Fatal("naturaltime: failed to parse timezones.json")
		return
	}

	// Uppercase all keys so lookups are case-insensitive.
	for k, v := range tzOffsets {
		upper := strings.ToUpper(k)
		if upper != k {
			tzOffsets[upper] = v
			delete(tzOffsets, k)
		}
	}

	// Build the default location from the EDT entry in the map.
	// This is used when no timezone is found in the input string.
	if edtOffset, ok := tzOffsets["EDT"]; ok {
		defaultLocation = time.FixedZone("EDT", edtOffset*60)
	} else {
		defaultLocation = time.UTC // last-resort safety net
	}

	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["parseNaturalTime"] = tmplParseNaturalTime(ctx)
	})
}

// parseTimezone attempts to extract a timezone from the input string.
// It checks for UTC/GMT offset expressions first (e.g. "UTC+1", "GMT-5:30"),
// then scans whitespace-separated tokens against the known abbreviations map
// (e.g. "EDT", "CEST").
// Returns the resolved location and the input string with the timezone removed.
func parseTimezone(input string) (*time.Location, string) {
	// Check for UTC+N / GMT-N style offsets first.
	if m := utcOffsetPattern.FindStringSubmatchIndex(input); m != nil {
		full := input[m[0]:m[1]]
		sign := input[m[4]:m[5]]
		hoursStr := input[m[6]:m[7]]

		hours, _ := strconv.Atoi(hoursStr)
		minutes := 0
		if m[8] != -1 {
			minutes, _ = strconv.Atoi(input[m[8]:m[9]])
		}

		offsetMinutes := hours*60 + minutes
		if sign == "-" {
			offsetMinutes = -offsetMinutes
		}

		loc := time.FixedZone(full, offsetMinutes*60)
		cleaned := strings.TrimSpace(utcOffsetPattern.ReplaceAllString(input, ""))
		return loc, cleaned
	}

	// Scan whitespace-separated tokens for a known timezone abbreviation.
	for token := range strings.FieldsSeq(input) {
		upper := strings.ToUpper(token)
		if offsetMinutes, ok := tzOffsets[upper]; ok {
			loc := time.FixedZone(upper, offsetMinutes*60)
			cleaned := strings.TrimSpace(strings.Replace(input, token, "", 1))
			return loc, cleaned
		}
	}

	return defaultLocation, input
}

// tmplParseNaturalTime parses a natural language time string, automatically
// detecting any timezone abbreviation or UTC/GMT offset in the input.
// Falls back to EDT if no timezone is found.
// Returns a time.Time in UTC.
//
// Counts against the template's generic API-call budget (100 calls per execution).
//
// Usage in a custom command:
//
//	{{$t := parseNaturalTime "tomorrow at 9pm EDT"}}
//	{{$t := parseNaturalTime "friday 3pm UTC+1"}}
//	{{$t := parseNaturalTime "next monday 10am GMT-5:30"}}
//	{{$t.Format "2006-01-02T15:04:05Z"}}
func tmplParseNaturalTime(ctx *templates.Context) interface{} {
	return func(input string) (time.Time, error) {
		if ctx.IncreaseCheckGenericAPICall() {
			return time.Time{}, templates.ErrTooManyAPICalls
		}

		loc, cleaned := parseTimezone(input)

		base := time.Now().In(loc)
		r, err := when.EN.Parse(cleaned, base)
		if err != nil {
			return time.Time{}, err
		}
		if r == nil {
			return time.Time{}, fmt.Errorf("could not understand time: %q", input)
		}

		return r.Time.UTC().Truncate(time.Minute), nil
	}
}
