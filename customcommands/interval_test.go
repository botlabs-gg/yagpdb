package customcommands

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/robfig/cron/v3"
	"github.com/volatiletech/null/v8"
)

// Common test helpers.

type expectedTime *time.Time

func testHelper(t *testing.T, testDescription string, got time.Time, want expectedTime, refT time.Time) {
	const layout = "Mon Jan 2 15:04 2006"

	t.Helper()
	if want == nil {
		if !got.IsZero() {
			t.Errorf("\n%s\ngot:\n\t%s\nwant:\n\tshould never run  (ref time: %s)", testDescription, got.Format(layout), refT.Format(layout))
		}
	} else {
		want := (*want).UTC()
		if !got.Equal(want) {
			gotPretty := got.Format(layout)
			if got.IsZero() {
				gotPretty = "never runs"
			}

			t.Errorf("\n%s\ngot:\n\t%s\nwant:\n\t%s  (ref time: %s)", testDescription, gotPretty, want.Format(layout), refT.Format(layout))
		}
	}
}

func runAt(t time.Time) expectedTime {
	t = t.UTC()
	return expectedTime(&t)
}

// runOnDate produces an expected time with the given date.
// Unspecified fields take on the values of the zero time.Time.
func runOnDate(opts ...dateOpt) expectedTime {
	date := date{Year: 1, Month: time.January, Day: 1, Hour: 0, Min: 0}
	for _, opt := range opts {
		opt(&date)
	}
	t := time.Date(date.Year, date.Month, date.Day, date.Hour, date.Min, 0, 0, time.UTC)
	return &t
}

func neverRuns() expectedTime {
	return nil
}

type date struct {
	Year           int
	Month          time.Month
	Day, Hour, Min int
}

type dateOpt func(*date)

func year(y int) dateOpt         { return func(d *date) { d.Year = y } }
func month(m time.Month) dateOpt { return func(d *date) { d.Month = m } }
func day(day int) dateOpt        { return func(d *date) { d.Day = day } }
func hour(h int) dateOpt         { return func(d *date) { d.Hour = h } }
func minute(min int) dateOpt     { return func(d *date) { d.Min = min } }

type exclude struct {
	Days  []time.Weekday
	Hours []int64
}

func allDays() []time.Weekday {
	return []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
}

func allHours() []int64 {
	var out []int64
	for h := range int64(24) {
		out = append(out, h)
	}
	return out
}
func allHoursExcept(hs ...int64) []int64 {
	var out []int64
	for h := range int64(24) {
		if !slices.Contains(hs, h) {
			out = append(out, h)
		}
	}
	return out
}

func TestIntervalNextRunTime(t *testing.T) {
	var now = time.Now().UTC()
	var zero = time.Time{}.UTC()

	type testcase struct {
		Name     string
		Interval int
		Exclude  exclude
		LastRun  null.Time
		RefTime  time.Time

		Want expectedTime
	}
	tests := []testcase{
		{Name: "basic 5m interval",
			Interval: 5,
			LastRun:  null.TimeFrom(now),
			RefTime:  now,
			Want:     runAt(now.Add(5 * time.Minute))},

		{Name: "never executed before runs immediately",
			Interval: 5,
			LastRun:  null.Time{},
			RefTime:  now,
			Want:     runAt(now)},

		{Name: "5m interval excluding hours",
			Interval: 5,
			Exclude:  exclude{Hours: []int64{0, 1}},
			LastRun:  null.TimeFrom(zero),
			RefTime:  zero,
			Want:     runOnDate(hour(2), minute(5))},

		{Name: "5m interval excluding days",
			Interval: 5,
			Exclude:  exclude{Days: []time.Weekday{time.Sunday, time.Monday}},
			LastRun:  null.TimeFrom(zero),
			RefTime:  zero,
			Want:     runOnDate(day(2))},

		{Name: "5m interval excluding days and hours",
			Interval: 5,
			Exclude:  exclude{Days: []time.Weekday{time.Tuesday}, Hours: allHoursExcept(1)},
			LastRun:  null.TimeFrom(zero.Add(2 * time.Hour)),
			RefTime:  zero.Add(2 * time.Hour),
			Want:     runOnDate(day(3), hour(1), minute(0))},

		{Name: "exclude all hours",
			Interval: 5,
			Exclude:  exclude{Hours: allHours()},
			LastRun:  null.Time{},
			RefTime:  now,
			Want:     neverRuns()},

		{Name: "exclude all days",
			Interval: 5,
			Exclude:  exclude{Days: allDays()},
			LastRun:  null.Time{},
			RefTime:  now,
			Want:     neverRuns()},

		{Name: "interval <5m never runs",
			Interval: 1,
			LastRun:  null.Time{},
			RefTime:  now,
			Want:     neverRuns()},

		{Name: "interval >31d never runs",
			Interval: 31*24*60 + 1,
			LastRun:  null.Time{},
			RefTime:  now,
			Want:     neverRuns()},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			days := make([]int64, len(tt.Exclude.Days))
			for i, v := range tt.Exclude.Days {
				days[i] = int64(v)
			}
			cmd := &models.CustomCommand{
				TriggerType:               int(CommandTriggerInterval),
				TimeTriggerInterval:       tt.Interval,
				TimeTriggerExcludingDays:  days,
				TimeTriggerExcludingHours: tt.Exclude.Hours,
				LastRun:                   tt.LastRun,
			}
			got := CalcNextRunTime(cmd, tt.RefTime).UTC()

			testDesc := fmt.Sprintf("interval every %d mins\n\texclude hours: %v\n\texclude days: %v",
				tt.Interval, tt.Exclude.Hours, tt.Exclude.Days)
			testHelper(t, testDesc, got, tt.Want, tt.RefTime)
		})
	}
}

func TestCronNextRunTime(t *testing.T) {
	var now = time.Now().UTC()
	var zero = time.Time{}.UTC()

	tests := []struct {
		Name    string
		Cron    string
		Exclude exclude
		RefTime time.Time

		Want expectedTime
	}{
		{Name: "basic on minute 5",
			Cron:    "5 * * * *",
			RefTime: zero.Add(3 * time.Minute),
			Want:    runOnDate(minute(5))},

		{Name: "on minute 0 of day 2",
			Cron:    "0 * 2 * *",
			RefTime: zero,
			Want:    runOnDate(day(2), hour(0), minute(0))},

		{Name: "on minute 0 of feb",
			Cron:    "0 * * 2 *",
			RefTime: zero,
			Want:    runOnDate(month(time.February), day(1), hour(0), minute(0))},

		{Name: "on minute 0 of tues",
			Cron:    "0 * * * 2",
			RefTime: zero,
			Want:    runOnDate(day(2), hour(0), minute(0))},

		{Name: "exclude all hours",
			Cron:    "* * * * *",
			Exclude: exclude{Hours: allHours()},
			RefTime: now,
			Want:    neverRuns()},

		{Name: "exclude hour 0 from cron running on hour 0",
			Cron:    "* 0 * * *",
			Exclude: exclude{Hours: []int64{0}},
			RefTime: now,
			Want:    neverRuns()},

		{Name: "exclude sunday from cron running on day 0",
			Cron:    "* * * * 0",
			Exclude: exclude{Days: []time.Weekday{time.Sunday}},
			RefTime: now,
			Want:    neverRuns()},

		{Name: "on minute 5 with exclude hours",
			Cron:    "5 * * * *",
			Exclude: exclude{Hours: []int64{0, 1}},
			RefTime: zero.Add(15 * time.Minute),
			Want:    runOnDate(hour(2), minute(5))},

		{Name: "on minute 0 on 2nd day with exclude hours",
			Cron:    "0 * 2 * *",
			Exclude: exclude{Hours: []int64{0, 1}},
			RefTime: zero.Add(15 * time.Minute),
			Want:    runOnDate(day(2), hour(2), minute(0))},

		{Name: "on minute 0 in february with exclude hours",
			Cron:    "0 * * 2 *",
			Exclude: exclude{Hours: []int64{0, 1}},
			RefTime: zero,
			Want:    runOnDate(month(time.February), day(1), hour(2), minute(0))},

		{Name: "on minute 0 on tues with exclude hours",
			Cron:    "0 * * * 2",
			Exclude: exclude{Hours: []int64{0, 1}},
			RefTime: zero,
			Want:    runOnDate(day(2), hour(2), minute(0))},

		{Name: "on minute 5 with exclude days",
			Cron:    "5 * * * *",
			Exclude: exclude{Days: []time.Weekday{time.Sunday, time.Monday}},
			RefTime: zero,
			Want:    runOnDate(day(2), hour(0), minute(5))},

		{Name: "on minute 0 on day 2,3 with exclude days",
			Cron:    "0 * 2,3 * *",
			Exclude: exclude{Days: []time.Weekday{time.Monday, time.Tuesday}},
			RefTime: zero,
			Want:    runOnDate(day(3), hour(0), minute(0))},

		{Name: "on minute 0 of day 1 in feb,mar,apr with exclude days",
			Cron:    "0 * 1 2,3,4 *",
			Exclude: exclude{Days: []time.Weekday{time.Thursday}},
			RefTime: zero,
			Want:    runOnDate(month(time.April), day(1), hour(0), minute(0))},

		{Name: "on minute 0 on tues,wed with exclude days",
			Cron:    "0 * * * 2,3",
			Exclude: exclude{Days: []time.Weekday{time.Tuesday}},
			RefTime: zero,
			Want:    runOnDate(day(3), hour(0), minute(0))},

		{Name: "on minute 5 with exclude days and hours",
			Cron:    "5 * * * *",
			Exclude: exclude{Days: []time.Weekday{time.Tuesday}, Hours: allHoursExcept(1)},
			RefTime: zero.Add(2 * time.Hour),
			Want:    runOnDate(day(3), hour(1), minute(5))},

		{Name: "on minute 0 of day 1,2,3 with exclude days and hours",
			Cron:    "0 * 1,2,3 * *",
			Exclude: exclude{Days: []time.Weekday{time.Tuesday}, Hours: allHoursExcept(1)},
			RefTime: zero.Add(2 * time.Hour),
			Want:    runOnDate(day(3), hour(1), minute(0))},

		{Name: "on minute 0 of first day of jan,feb,mar,apr with exclude days and hours",
			Cron:    "0 * 1 1,2,3,4 *",
			Exclude: exclude{Days: []time.Weekday{time.Thursday}, Hours: allHoursExcept(1)},
			RefTime: zero.Add(2 * time.Hour),
			Want:    runOnDate(month(time.April), day(1), hour(1), minute(0))},

		{Name: "on minute 0 of mon,tue,wed with exclude days and hours",
			Cron:    "0 * * * 1,2,3",
			Exclude: exclude{Days: []time.Weekday{time.Tuesday}, Hours: allHoursExcept(1)},
			RefTime: zero.Add(2 * time.Hour),
			Want:    runOnDate(day(3), hour(1), minute(0))},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(tt.Cron)
			if err != nil {
				t.Errorf("invalid cron expression %q in test: %s", tt.Cron, err)
			}

			days := make([]int64, len(tt.Exclude.Days))
			for i, v := range tt.Exclude.Days {
				days[i] = int64(v)
			}
			cmd := &models.CustomCommand{
				TriggerType:               int(CommandTriggerCron),
				TextTrigger:               tt.Cron,
				TimeTriggerExcludingDays:  days,
				TimeTriggerExcludingHours: tt.Exclude.Hours,
			}

			got := CalcNextRunTime(cmd, tt.RefTime).UTC()

			testDesc := fmt.Sprintf("cron %q\n\texclude hours: %v\n\texclude days: %v", tt.Cron, tt.Exclude.Hours, tt.Exclude.Days)
			testHelper(t, testDesc, got, tt.Want, tt.RefTime)
		})
	}
}
