package customcommands

import (
	"context"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	scheduledmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/sqlboiler/queries/qm"
	// evtsmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/jonas747/yagpdb/customcommands/models"
	"github.com/pkg/errors"
	// "github.com/volatiletech/sqlboiler/queries/qm"
	"time"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["parseArgs"] = tmplExpectArgs(ctx)
		ctx.ContextFuncs["carg"] = tmplCArg
		ctx.ContextFuncs["execCC"] = tmplRunCC(ctx)
		ctx.ContextFuncs["scheduleUniqueCC"] = tmplScheduleUniqueCC(ctx)
		ctx.ContextFuncs["cancelScheduledUniqueCC"] = tmplCancelUniqueCC(ctx)
	})
}

func tmplCArg(typ string, name string, opts ...interface{}) (*dcmd.ArgDef, error) {
	def := &dcmd.ArgDef{Name: name}
	switch typ {
	case "int":
		if len(opts) >= 2 {
			def.Type = &dcmd.IntArg{Min: templates.ToInt64(opts[0]), Max: templates.ToInt64(opts[1])}
		} else {
			def.Type = dcmd.Int
		}
	case "duration":
		if len(opts) >= 2 {
			def.Type = &commands.DurationArg{Min: time.Duration(templates.ToInt64(opts[0])), Max: time.Duration(templates.ToInt64(opts[1]))}
		} else {
			def.Type = &commands.DurationArg{}
		}
	case "string":
		def.Type = dcmd.String
	case "user":
		def.Type = dcmd.UserReqMention
	case "userid":
		def.Type = dcmd.UserID
	case "channel":
		def.Type = dcmd.Channel
	case "member":
		def.Type = &commands.MemberArg{}
	default:
		return nil, errors.New("Unknown type")
	}

	return def, nil
}

func tmplExpectArgs(ctx *templates.Context) interface{} {
	return func(numRequired int, failedMessage string, args ...*dcmd.ArgDef) (*ParsedArgs, error) {
		result := &ParsedArgs{}
		if len(args) == 0 || ctx.Msg == nil || ctx.Data["StrippedMsg"] == nil {
			return result, nil
		}

		result.defs = args

		ctxMember := ctx.MS

		msg := ctx.Msg
		stripped := ctx.Data["StrippedMsg"].(string)
		split := dcmd.SplitArgs(stripped)

		// create the dcmd data context used in the arg parsing
		dcmdData, err := commands.CommandSystem.FillData(common.BotSession, msg)
		if err != nil {
			return result, errors.WithMessage(err, "tmplExpectArgs")
		}

		dcmdData.MsgStrippedPrefix = stripped
		dcmdData = dcmdData.WithContext(context.WithValue(dcmdData.Context(), commands.CtxKeyMS, ctxMember))

		// attempt to parse them
		err = dcmd.ParseArgDefs(args, numRequired, nil, dcmdData, split)
		if err != nil {
			if failedMessage != "" {
				ctx.FixedOutput = err.Error() + "\n" + failedMessage
			} else {
				ctx.FixedOutput = err.Error() + "\nUsage: `" + (*dcmd.StdHelpFormatter).ArgDefLine(nil, args, numRequired) + "`"
			}
		}
		result.parsed = dcmdData.Args
		return result, err
	}
}

type ParsedArgs struct {
	defs   []*dcmd.ArgDef
	parsed []*dcmd.ParsedArg
}

func (pa *ParsedArgs) Get(index int) interface{} {
	if len(pa.parsed) <= index {
		return nil
	}

	switch pa.parsed[index].Def.Type.(type) {
	case *dcmd.IntArg:
		return pa.parsed[index].Int()
	case *dcmd.ChannelArg:
		i := pa.parsed[index].Value
		if i == nil {
			return nil
		}

		c := i.(*dstate.ChannelState)
		c.Owner.RLock()
		cop := c.DGoCopy()
		c.Owner.RUnlock()
		return cop
	case *commands.MemberArg:
		i := pa.parsed[index].Value
		if i == nil {
			return nil
		}

		m := i.(*dstate.MemberState)
		return m.DGoCopy()
	}

	return pa.parsed[index].Value
}

func (pa *ParsedArgs) IsSet(index int) interface{} {
	return pa.Get(index) != nil
}

// tmplRunCC either run another custom command immeditely with a max stack depth of 2
// or schedules a custom command to be run in the future sometime with the provided data placed in .ExecData
func tmplRunCC(ctx *templates.Context) interface{} {
	return func(ccID int, channel interface{}, delaySeconds interface{}, data interface{}) (string, error) {
		if ctx.IncreaseCheckCallCounter("runcc", 1) {
			return "", templates.ErrTooManyCalls
		}

		cmd, err := models.FindCustomCommandG(context.Background(), ctx.GS.ID, int64(ccID))
		if err != nil {
			return "", errors.New("Couldn't find custom command")
		}

		channelID := ctx.ChannelArg(channel)
		if channelID == 0 {
			return "", errors.New("Unknown channel")
		}

		cs := ctx.GS.Channel(true, channelID)
		if cs == nil {
			return "", errors.New("Channel not in state")
		}

		actualDelay := templates.ToInt64(delaySeconds)
		if actualDelay <= 0 {
			currentStackDepthI := ctx.Data["StackDepth"]
			currentStackDepth := 0
			if currentStackDepthI != nil {
				currentStackDepth = currentStackDepthI.(int)
			}

			if currentStackDepth >= 2 {
				return "", errors.New("Max nested immediate execCC calls reached (2)")
			}

			newCtx := templates.NewContext(ctx.GS, cs, ctx.MS)
			if ctx.Msg != nil {
				newCtx.Msg = ctx.Msg
				newCtx.Data["Message"] = ctx.Msg
			}
			newCtx.Data["ExecData"] = data
			newCtx.Data["StackDepth"] = currentStackDepth + 1

			go ExecuteCustomCommand(cmd, newCtx)
			return "", nil
		}

		m := &DelayedRunCCData{
			ChannelID: channelID,
			CmdID:     cmd.LocalID,

			Member:  ctx.MS,
			Message: ctx.Msg,
		}

		// embed data using msgpack to include type information
		if data != nil {
			encoded, err := msgpack.Marshal(data)
			if err != nil {
				return "", err
			}

			m.UserData = encoded
		}

		err = scheduledevents2.ScheduleEvent("cc_delayed_run", ctx.GS.ID, time.Now().Add(time.Second*time.Duration(actualDelay)), m)
		if err != nil {
			return "", errors.Wrap(err, "failed scheduling cc run")
		}

		return "", nil
	}
}

// tmplScheduleUniqueCC schedules a custom command to be ran in the future, but you can provide a key where it will overwrite existing
// scheduled runs with the same cc id and key
//
// for example in a custom mute command you only want 1 scheduled unmute cc per user, to do that you would use the userid as the key
// then when you use the custom mute command again it will overwrite the mute duration and overwrite the scheduled unmute cc for that user
func tmplScheduleUniqueCC(ctx *templates.Context) interface{} {
	return func(ccID int, channel interface{}, delaySeconds interface{}, key interface{}, data interface{}) (string, error) {
		if ctx.IncreaseCheckCallCounter("runcc", 1) {
			return "", templates.ErrTooManyCalls
		}

		cmd, err := models.FindCustomCommandG(context.Background(), ctx.GS.ID, int64(ccID))
		if err != nil {
			return "", errors.New("Couldn't find custom command")
		}

		channelID := ctx.ChannelArg(channel)
		if channelID == 0 {
			return "", errors.New("Unknown channel")
		}

		cs := ctx.GS.Channel(true, channelID)
		if cs == nil {
			return "", errors.New("Channel not in state")
		}

		actualDelay := templates.ToInt64(delaySeconds)
		if actualDelay <= 0 {
			return "", nil
		}

		stringedKey := templates.ToString(key)

		m := &DelayedRunCCData{
			ChannelID: channelID,
			CmdID:     cmd.LocalID,

			Member:  ctx.MS,
			Message: ctx.Msg,
			UserKey: stringedKey,
		}

		// embed data using msgpack to include type information
		if data != nil {
			encoded, err := msgpack.Marshal(data)
			if err != nil {
				return "", err
			}

			m.UserData = encoded
		}

		// since this is a unique, remove existing ones
		_, err = scheduledmodels.ScheduledEvents(
			qm.Where("event_name='cc_delayed_run' AND  guild_id = ? AND (data->>'user_key')::bigint = ? AND (data->>'cmd_id')::bigint = ? AND processed = false",
				ctx.GS.ID, stringedKey, cmd.LocalID)).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			return "", err
		}

		err = scheduledevents2.ScheduleEvent("cc_delayed_run", ctx.GS.ID, time.Now().Add(time.Second*time.Duration(actualDelay)), m)
		if err != nil {
			return "", errors.Wrap(err, "failed scheduling cc run")
		}

		return "", nil
	}
}

// tmplCancelUniqueCC cancels a scheduled cc execution in the future with the provided cc id and key
func tmplCancelUniqueCC(ctx *templates.Context) interface{} {
	return func(ccID int, key interface{}) (string, error) {
		if ctx.IncreaseCheckCallCounter("cancelcc", 2) {
			return "", templates.ErrTooManyCalls
		}

		stringedKey := templates.ToString(key)

		// since this is a unique, remove existing ones
		_, err := scheduledmodels.ScheduledEvents(
			qm.Where("event_name='cc_delayed_run' AND  guild_id = ? AND (data->>'user_key')::bigint = ? AND (data->>'cmd_id')::bigint = ? AND processed = false",
				ctx.GS.ID, stringedKey, ccID)).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			return "", err
		}

		return "", nil
	}
}
