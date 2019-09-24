package customcommands

import (
	"testing"
	"time"

	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/volatiletech/null"
)

func TestNextRunTimeBasic(t *testing.T) {
	cc := &models.CustomCommand{
		TimeTriggerInterval: 1,
	}

	tim := time.Now().UTC()

	nextRun := CalcNextRunTime(cc, tim)

	if tim != nextRun {
		t.Error("next run should be now: ", tim, ", not: ", nextRun)
	}

	tim = time.Time{}
	cc.LastRun = null.TimeFrom(tim)

	next := CalcNextRunTime(cc, tim)
	if next != tim.UTC().Add(time.Minute) {
		t.Error("incorrect next run, should be: ", tim.UTC().Add(time.Minute), ", got: ", next)
	}
}

func TestNextRunTimeImpossible(t *testing.T) {
	cc := &models.CustomCommand{
		TimeTriggerInterval:       1,
		TimeTriggerExcludingHours: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23},
	}

	next := CalcNextRunTime(cc, time.Now())
	if !next.IsZero() {
		t.Error("next time is not zero: ", next)
	}

	cc.TimeTriggerExcludingHours = nil
	cc.TimeTriggerExcludingDays = []int64{0, 1, 2, 3, 4, 5, 6}

	next = CalcNextRunTime(cc, time.Now())
	if !next.IsZero() {
		t.Error("next time is not zero: ", next)
	}
}

func TestNextRunTimeExcludingHours(t *testing.T) {
	cc := &models.CustomCommand{
		TimeTriggerInterval:       1,
		TimeTriggerExcludingHours: []int64{0, 1},
	}

	tim := time.Time{}

	nextRun := CalcNextRunTime(cc, tim)
	expected := tim.Add((time.Hour * 2) + time.Minute)

	if nextRun != expected {
		t.Error("next run should be now: ", expected, ", got: ", nextRun)
	}
}

func TestNextRunTimeExcludingDays(t *testing.T) {
	cc := &models.CustomCommand{
		TimeTriggerInterval:      1,
		TimeTriggerExcludingDays: []int64{0, 1},
	}

	tim := time.Time{}

	nextRun := CalcNextRunTime(cc, tim)
	expected := tim.Add(time.Hour * 24)

	if nextRun != expected {
		t.Error("next run should be now: ", expected, ", got: ", nextRun, tim.Weekday())
	}
}

func TestNextRunTimeExcludingDaysHours(t *testing.T) {
	cc := &models.CustomCommand{
		TimeTriggerInterval:       1,
		TimeTriggerExcludingDays:  []int64{2},
		TimeTriggerExcludingHours: []int64{0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23},
	}

	tim := time.Time{}
	tim = tim.UTC()
	tim = tim.Add(time.Hour * 2)

	nextRun := CalcNextRunTime(cc, tim)
	expected := tim.Add(time.Hour * 47)

	if nextRun != expected {
		t.Errorf("next run should be: %s (w:%d) got %s (w:%d - %d)", expected, expected.Weekday(), nextRun, int(nextRun.Weekday()), nextRun.Hour())
	}
}
