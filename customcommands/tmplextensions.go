package customcommands

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	scheduledmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/sqlboiler/boil"
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

		ctx.ContextFuncs["dbSet"] = tmplDBSet(ctx)
		ctx.ContextFuncs["dbSetExpire"] = tmplDBSetExpire(ctx)
		ctx.ContextFuncs["dbGet"] = tmplDBGet(ctx)
		ctx.ContextFuncs["dbDel"] = tmplDBDel(ctx)
		ctx.ContextFuncs["dbTopUsers"] = tmplDBTopUsers(ctx)
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

func tmplDBSet(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}, value interface{}) (string, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}

		valueSerialized, err := serializeValue(value)
		if err != nil {
			return "", err
		}

		vNum := templates.ToFloat64(value)
		keyStr := templates.ToString(key)

		m := &models.TemplatesUserDatabase{
			GuildID: ctx.GS.ID,
			UserID:  userID,

			Key:      keyStr,
			ValueRaw: valueSerialized,
			ValueNum: vNum,
		}

		err = m.Upsert(context.Background(), common.PQ, true, []string{"guild_id", "user_id", "key"}, boil.Whitelist("value_raw", "value_num"), boil.Infer())
		return "", err
	}
}

func tmplDBSetExpire(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}, value interface{}, ttl int) (string, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}

		return "", nil
	}
}

func tmplDBIncr(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}, incrBy interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}

		vNum := templates.ToFloat64(incrBy)
		valueSerialized, err := serializeValue(vNum)
		if err != nil {
			return "", err
		}

		keyStr := templates.ToString(key)

		const q = `INSERT INTO templates_user_database (guild_id, user_id, key, value_raw, value_num) 
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (guil_id, user_id, key) 
ON UPDATE SET value_num = templates_user_database.value_num + $5`

		_, err = common.PQ.Exec(q, ctx.GS.ID, userID, keyStr, valueSerialized, vNum)

		return "", err
	}
}

func tmplDBGet(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}

		keyStr := templates.ToString(key)
		m, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND user_id = ? AND key = ? AND (expires_at IS NULL OR expires_at > now())", ctx.GS.ID, userID, keyStr)).OneG(context.Background())
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}

			return nil, nil
		}

		var dst interface{}
		err = msgpack.Unmarshal(m.ValueRaw, &dst)
		if err != nil {
			return nil, err
		}

		if common.IsNumber(dst) {
			return m.ValueNum, nil
		}

		return dst, nil
	}
}

func tmplDBDel(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}
		keyStr := templates.ToString(key)
		_, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND user_id = ? AND key = ?", ctx.GS.ID, userID, keyStr)).DeleteAll(context.Background(), common.PQ)

		return "", err
	}
}

func tmplDBTopUsers(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounter("db_interactions", 10) {
			return "", templates.ErrTooManyCalls
		}

		return "", nil
	}
}

func serializeValue(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := msgpack.NewEncoder(templates.LimitWriter(&b, 100000))
	err := enc.Encode(v)
	return b.Bytes(), err
}

// returns true if were above db limit for the specified guild
func isAboveDBLimit(gs *dstate.GuildState) (bool, error) {
	gs.RLock()
	limit := gs.Guild.MemberCount * 10
	gs.RUnlock()

	count, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND (expires_at > now() OR expires_at IS NULL)", gs.ID)).CountG(context.Background())
	if err != nil {
		return false, err
	}

	return limit <= int(count), nil
}
