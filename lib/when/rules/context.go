package rules

import "time"

type Context struct {
	Text string

	// accumulator of relative values
	Duration time.Duration

	// Aboslute values
	Year, Month, Weekday, Day, Hour, Minute, Second *int

	Location *time.Location
}

func (c *Context) Time(t time.Time) (time.Time, error) {
	if t.IsZero() {
		t = time.Now()
	}

	if c.Duration != 0 {
		t = t.Add(c.Duration)
	}

	year, month, day := t.Date()

	if c.Year != nil {
		year = *c.Year
	}

	if c.Month != nil {
		month = time.Month(*c.Month)
	}

	if c.Day != nil {
		day = *c.Day
	}

	hour, min, sec := t.Clock()

	if c.Hour != nil {
		hour = *c.Hour
	}

	if c.Minute != nil {
		min = *c.Minute
	}

	if c.Second != nil {
		sec = *c.Second
	}

	loc := t.Location()

	if c.Location != nil {
		loc = c.Location
	}

	t = time.Date(year, month, day, hour,
		min, sec, 0, loc)

	if c.Weekday != nil {
		diff := int(time.Weekday(*c.Weekday) - t.Weekday())
		t = time.Date(t.Year(), t.Month(), t.Day()+diff, t.Hour(),
			t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	}

	return t, nil
}
