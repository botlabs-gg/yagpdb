package customcommands

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	schEventsModels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func handleCustomCommandsRunNow(event *pubsub.Event) {
	dataCast := event.Data.(*models.CustomCommand)
	f := logger.WithFields(logrus.Fields{
		"guild_id": dataCast.GuildID,
		"cmd_id":   dataCast.LocalID,
	})

	gs := bot.State.GetGuild(dataCast.GuildID)
	if gs == nil {
		f.Error("failed fetching active guild from state")
		return
	}

	cs := gs.GetChannel(dataCast.ContextChannel)
	if cs == nil {
		f.Error("failed finding channel to run cc in")
		return
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	tmplCtx := templates.NewContext(gs, cs, nil)
	ExecuteCustomCommand(dataCast, tmplCtx)

	dataCast.LastRun = null.TimeFrom(time.Now())
	err := UpdateCommandNextRunTime(dataCast, true, true)
	if err != nil {
		f.WithError(err).Error("failed updating custom command next run time")
	}
}

func handleDelayedRunCC(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	dataCast := data.(*DelayedRunCCData)
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, dataCast.CmdID), qm.Load("Group")).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	if cmd.R.Group != nil && cmd.R.Group.Disabled {
		return false, errors.New("custom command group is disabled")
	}

	if cmd.Disabled {
		return false, errors.New("custom command is disabled")
	}

	if !DelayedCCRunLimit.AllowN(DelayedRunLimitKey{GuildID: evt.GuildID, ChannelID: dataCast.ChannelID}, time.Now(), 1) {
		logger.WithField("guild", cmd.GuildID).Warn("went above delayed cc run ratelimit")
		return false, nil
	}

	gs := bot.State.GetGuild(evt.GuildID)
	if gs == nil {
		// in case the bot left in the meantime
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.GetChannelOrThread(dataCast.ChannelID)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.Available {
			return true, nil
		}

		return false, nil
	}

	// attempt to get up to date member information
	if dataCast.Member != nil {
		updatedMS, _ := bot.GetMember(gs.ID, dataCast.Member.User.ID)
		if updatedMS != nil {
			dataCast.Member = updatedMS
		}
	}

	tmplCtx := templates.NewContext(gs, cs, dataCast.Member)
	if dataCast.Message != nil {
		tmplCtx.Msg = dataCast.Message
		tmplCtx.Data["Message"] = dataCast.Message
	}

	tmplCtx.ExecutedFrom = dataCast.ExecutedFrom

	// decode userdata
	if len(dataCast.UserData) > 0 {
		var i interface{}
		err := msgpack.Unmarshal(dataCast.UserData, &i)
		if err != nil {
			return false, err
		}

		tmplCtx.Data["ExecData"] = i
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	err = ExecuteCustomCommand(cmd, tmplCtx)
	return false, err
}

func handleNextRunScheduledEVent(evt *schEventsModels.ScheduledEvent, data interface{}) (retry bool, err error) {
	cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", evt.GuildID, (data.(*NextRunScheduledEvent)).CmdID), qm.Load("Group")).OneG(context.Background())
	if err != nil {
		return false, errors.WrapIf(err, "find_command")
	}

	if cmd.R.Group != nil && cmd.R.Group.Disabled {
		return false, errors.New("custom command group is disabled")
	}

	if cmd.Disabled {
		return false, errors.New("custom command is disabled")
	}

	if time.Until(cmd.NextRun.Time) > time.Second*5 {
		return false, nil // old scheduled event that wasn't removed, /shrug

	}

	gs := bot.State.GetGuild(evt.GuildID)
	if gs == nil {
		if onGuild, err := common.BotIsOnGuild(evt.GuildID); !onGuild && err == nil {
			return false, nil
		} else if err != nil {
			logger.WithError(err).Error("failed checking if bot is on guild")
		}

		return true, nil
	}

	cs := gs.GetChannel(cmd.ContextChannel)
	if cs == nil {
		// don't reschedule if channel is deleted, make sure its actually not there, and not just a discord downtime
		if !gs.Available {
			return true, nil
		}

		return false, nil
	}

	metricsExecutedCommands.With(prometheus.Labels{"trigger": "timed"}).Inc()

	tmplCtx := templates.NewContext(gs, cs, nil)
	ExecuteCustomCommand(cmd, tmplCtx)

	// schedule next runs
	cmd.LastRun = cmd.NextRun
	err = UpdateCommandNextRunTime(cmd, true, false)
	if err != nil {
		logger.WithError(err).Error("failed updating custom command next run time")
	}

	return false, nil
}
