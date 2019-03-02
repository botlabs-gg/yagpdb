package customcommands

import (
	"context"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
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
	default:
		return nil, errors.New("Unknown type")
	}

	return def, nil
}

func tmplExpectArgs(ctx *templates.Context) interface{} {
	return func(numRequired int, failedMessage string, args ...*dcmd.ArgDef) (*ParsedArgs, error) {
		result := &ParsedArgs{}
		if len(args) == 0 {
			return result, nil
		}

		result.Defs = args

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
		result.Parsed = dcmdData.Args
		return result, err
	}
}

type ParsedArgs struct {
	Defs   []*dcmd.ArgDef
	Parsed []*dcmd.ParsedArg
}

func (pa *ParsedArgs) Get(index int) interface{} {
	if len(pa.Parsed) <= index {
		return nil
	}

	switch pa.Parsed[index].Def.Type.(type) {
	case *dcmd.IntArg:
		return pa.Parsed[index].Int()
	case *dcmd.ChannelArg:
		c := pa.Parsed[index].Value.(*dstate.ChannelState)
		c.Owner.RLock()
		cop := c.DGoCopy()
		c.Owner.RUnlock()
		return cop
	}

	return pa.Parsed[index].Value
}

func (pa *ParsedArgs) IsSet(index int) interface{} {
	return pa.Get(index) != nil
}

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
			UserData:  data,

			Member:  ctx.MS,
			Message: ctx.Msg,
		}

		err = scheduledevents2.ScheduleEvent("cc_delayed_run", ctx.GS.ID, time.Now().Add(time.Second*time.Duration(actualDelay)), m)
		if err != nil {
			return "", errors.Wrap(err, "failed scheduling cc run")
		}

		return "", nil
	}
}
