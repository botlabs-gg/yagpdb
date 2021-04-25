package customcommands

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	schEventsModels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

// CalcNextRunTime calculates the next run time for a custom command using the last ran time
func CalcNextRunTime(cc *models.CustomCommand, now time.Time) time.Time {
	if len(cc.TimeTriggerExcludingDays) >= 7 || len(cc.TimeTriggerExcludingHours) >= 24 {
		// this can never be ran...
		return time.Time{}
	}

	tNext := cc.LastRun.Time.Add(time.Minute * time.Duration(cc.TimeTriggerInterval))
	// run it immedietely if this is the case
	if tNext.Before(now) {
		tNext = now
	}

	// ensure were dealing with utc
	tNext = tNext.UTC()

	// Check for blaclisted days and if we encountered a blacklisted day we reset the clock
	tNext = intervalCheckDays(cc, tNext, true)

	// check for blacklisted hours
	tNext = intervalCheckHours(cc, tNext)

	// its possible we went forward a day while checking for blacklisted hours, in that case check for blacklisted days again
	tNext = intervalCheckDays(cc, tNext, true)

	// AND finally if we
	// 1. blacklisted hour led to nextTime going to another day
	// 2. hour is now reset
	// 3. hour is now blacklisted
	// do a final check for #3
	//
	// should not be possible to land on a new day after this, so further checks are not needed
	tNext = intervalCheckHours(cc, tNext)

	return tNext
}

func intervalCheckDays(cc *models.CustomCommand, tNext time.Time, resetClock bool) time.Time {
	// check for blacklisted days
	if !common.ContainsInt64Slice(cc.TimeTriggerExcludingDays, int64(tNext.Weekday())) {
		return tNext
	}

	// find the next available day
	for {
		tNext = tNext.Add(time.Hour * 24)

		if !common.ContainsInt64Slice(cc.TimeTriggerExcludingDays, int64(tNext.Weekday())) {
			break
		}
	}

	if resetClock {
		// if we went forward a day, force the clock to 0 to run it as soon as possible
		h, m, s := tNext.Clock()
		tNext = tNext.Add((-time.Hour * time.Duration(h)))
		tNext = tNext.Add((-time.Minute * time.Duration(m)))
		tNext = tNext.Add((-time.Second * time.Duration(s)))
	}
	return tNext
}

func intervalCheckHours(cc *models.CustomCommand, tNext time.Time) time.Time {
	// check for blacklisted hours
	if !common.ContainsInt64Slice(cc.TimeTriggerExcludingHours, int64(tNext.Hour())) {
		return tNext
	}

	// find the next available hour
	for {
		tNext = tNext.Add(time.Hour)

		if !common.ContainsInt64Slice(cc.TimeTriggerExcludingHours, int64(tNext.Hour())) {
			break
		}
	}

	return tNext
}

type NextRunScheduledEvent struct {
	CmdID int64 `json:"cmd_id"`
}

func DelNextRunEvent(guildID int64, cmdID int64) error {
	_, err := schEventsModels.ScheduledEvents(qm.Where("event_name='cc_next_run' AND guild_id = ? AND (data->>'cmd_id')::bigint = ?", guildID, cmdID)).DeleteAll(context.Background(), common.PQ)
	return err
}

// TODO: Run this all in a transaction?
func UpdateCommandNextRunTime(cc *models.CustomCommand, updateLastRun bool, clearOld bool) error {
	if clearOld {
		// remove the old events
		err := DelNextRunEvent(cc.GuildID, cc.LocalID)
		if err != nil {
			return errors.WrapIf(err, "del_old_events")
		}
	}

	if cc.TriggerType != int(CommandTriggerInterval) || cc.TimeTriggerInterval < 1 {
		return nil
	}

	// calculate the next run time
	nextRun := CalcNextRunTime(cc, time.Now())
	if nextRun.IsZero() {
		return nil
	}

	// update the command
	cc.NextRun = null.TimeFrom(nextRun)
	toUpdate := []string{"next_run"}
	if updateLastRun {
		toUpdate = append(toUpdate, "last_run")
	}
	_, err := cc.UpdateG(context.Background(), boil.Whitelist(toUpdate...))
	if err != nil {
		return errors.WrapIf(err, "update_cc")
	}

	evt := &NextRunScheduledEvent{
		CmdID: cc.LocalID,
	}

	// create a scheduled event to run the command again
	err = scheduledevents2.ScheduleEvent("cc_next_run", cc.GuildID, nextRun, evt)
	if err != nil {
		return errors.WrapIf(err, "schedule_event")
	}

	return nil
}
