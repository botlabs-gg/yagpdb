package customcommands

import (
	"bytes"
	"context"
	"database/sql"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	scheduledmodels "github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2/models"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/customcommands/models"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/vmihailenco/msgpack"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
		ctx.ContextFuncs["dbIncr"] = tmplDBIncr(ctx)
		ctx.ContextFuncs["dbGet"] = tmplDBGet(ctx)
		ctx.ContextFuncs["dbGetPattern"] = tmplDBGetPattern(ctx, false)
		ctx.ContextFuncs["dbGetPatternReverse"] = tmplDBGetPattern(ctx, true)
		ctx.ContextFuncs["dbDel"] = tmplDBDel(ctx)
		ctx.ContextFuncs["dbDelById"] = tmplDBDelById(ctx)
		ctx.ContextFuncs["dbDelByID"] = tmplDBDelById(ctx)
		ctx.ContextFuncs["dbDelMultiple"] = tmplDBDelMultiple(ctx)
		ctx.ContextFuncs["dbTopEntries"] = tmplDBTopEntries(ctx, false)
		ctx.ContextFuncs["dbBottomEntries"] = tmplDBTopEntries(ctx, true)
		ctx.ContextFuncs["dbCount"] = tmplDBCount(ctx)
		ctx.ContextFuncs["dbRank"] = tmplDBRank(ctx)
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
	case "float":
		if len(opts) >= 2 {
			def.Type = &dcmd.FloatArg{Min: templates.ToFloat64(opts[0]), Max: templates.ToFloat64(opts[1])}
		} else {
			def.Type = dcmd.Float
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
		def.Type = dcmd.User
	case "userid":
		def.Type = dcmd.UserID
	case "channel":
		def.Type = dcmd.ChannelOrThread
	case "member":
		def.Type = &commands.MemberArg{}
	case "role":
		def.Type = &commands.RoleArg{}
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

		msg := ctx.Msg
		stripped := ctx.Data["StrippedMsg"].(string)
		split := dcmd.SplitArgs(stripped)

		// create the dcmd data context used in the arg parsing
		dcmdData, err := commands.CommandSystem.FillDataLegacyMessage(common.BotSession, msg)
		if err != nil {
			return result, errors.WithMessage(err, "tmplExpectArgs")
		}

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
		i := pa.parsed[index]
		if i.Value == nil {
			return nil
		}
		return i.Int()
	case *dcmd.ChannelArg:
		i := pa.parsed[index].Value
		if i == nil {
			return nil
		}

		c := i.(*dstate.ChannelState)
		return templates.CtxChannelFromCS(c)
	case *commands.MemberArg:
		i := pa.parsed[index].Value
		if i == nil {
			return nil
		}

		m := i.(*dstate.MemberState)
		return m.DgoMember()
	case *commands.RoleArg:
		i := pa.parsed[index].Value
		if i == nil {
			return nil
		}

		return i.(*discordgo.Role)
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
		if ctx.ExecutedFrom == templates.ExecutedFromNestedCommandTemplate {
			return "", nil
		}

		if ctx.IncreaseCheckCallCounterPremium("runcc", 1, 10) {
			return "", templates.ErrTooManyCalls
		}

		cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", ctx.GS.ID, ccID), qm.Load("Group")).OneG(context.Background())
		if err != nil {
			return "", errors.New("Couldn't find custom command")
		}

		if cmd.R.Group != nil && cmd.R.Group.Disabled {
			return "", errors.New("custom command group is disabled")
		}

		if cmd.Disabled {
			return "", errors.New("custom command is disabled")
		}

		if cmd.TriggerType == int(CommandTriggerInterval) || cmd.TriggerType == int(CommandTriggerCron) {
			return "", errors.New("interval and cron type custom commands cannot be used with execCC")
		}

		channelID := ctx.ChannelArg(channel)
		if channelID == 0 {
			return "", errors.New("Unknown channel")
		}

		cs := ctx.GS.GetChannelOrThread(channelID)
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
			if ctx.CurrentFrame.Interaction != nil {
				newCtx.CurrentFrame.Interaction = ctx.CurrentFrame.Interaction
				newCtx.Data["Interaction"] = ctx.CurrentFrame.Interaction
			}
			newCtx.Data["ExecData"] = data
			newCtx.Data["StackDepth"] = currentStackDepth + 1
			newCtx.ExecutedFrom = ctx.ExecutedFrom

			go ExecuteCustomCommand(cmd, newCtx)
			return "", nil
		}

		m := &DelayedRunCCData{
			ChannelID: channelID,
			CmdID:     cmd.LocalID,

			Member:  ctx.MS,
			Message: ctx.Msg,

			ExecutedFrom: ctx.ExecutedFrom,
		}

		// embed data using msgpack to include type information
		if data != nil {
			encoded, err := msgpack.Marshal(data)
			if err != nil {
				return "", err
			}

			m.UserData = encoded
			if len(m.UserData) > CCMaxDataLimit {
				return "", errors.New("ExecData is too big")
			}
		}

		err = scheduledevents2.ScheduleEvent("cc_delayed_run", ctx.GS.ID, time.Now().Add(time.Second*time.Duration(actualDelay)), m)
		if err != nil {
			return "", errors.WrapIf(err, "failed scheduling cc run")
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
		if ctx.IncreaseCheckCallCounterPremium("runcc", 1, 10) {
			return "", templates.ErrTooManyCalls
		}

		cmd, err := models.CustomCommands(qm.Where("guild_id = ? AND local_id = ?", ctx.GS.ID, ccID), qm.Load("Group")).OneG(context.Background())
		if err != nil {
			return "", errors.New("Couldn't find custom command")
		}

		if cmd.R.Group != nil && cmd.R.Group.Disabled {
			return "", errors.New("custom command group is disabled")
		}

		if cmd.Disabled {
			return "", errors.New("custom command is disabled")
		}

		if cmd.TriggerType == int(CommandTriggerInterval) || cmd.TriggerType == int(CommandTriggerCron) {
			return "", errors.New("interval and cron type custom commands cannot be used with scheduleUniqueCC")
		}

		channelID := ctx.ChannelArg(channel)
		if channelID == 0 {
			return "", errors.New("Unknown channel")
		}

		cs := ctx.GS.GetChannelOrThread(channelID)
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

			ExecutedFrom: ctx.ExecutedFrom,
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
			qm.Where("event_name='cc_delayed_run' AND  guild_id = ? AND (data->>'user_key')::text = ? AND (data->>'cmd_id')::bigint = ? AND processed = false",
				ctx.GS.ID, stringedKey, cmd.LocalID)).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			return "", err
		}

		err = scheduledevents2.ScheduleEvent("cc_delayed_run", ctx.GS.ID, time.Now().Add(time.Second*time.Duration(actualDelay)), m)
		if err != nil {
			return "", errors.WrapIf(err, "failed scheduling cc run")
		}

		return "", nil
	}
}

// tmplCancelUniqueCC cancels a scheduled cc execution in the future with the provided cc id and key
func tmplCancelUniqueCC(ctx *templates.Context) interface{} {
	return func(ccID int, key interface{}) (string, error) {
		if ctx.IncreaseCheckCallCounter("cancelcc", 10) {
			return "", templates.ErrTooManyCalls
		}

		stringedKey := templates.ToString(key)

		// since this is a unique, remove existing ones
		_, err := scheduledmodels.ScheduledEvents(
			qm.Where("event_name='cc_delayed_run' AND  guild_id = ? AND (data->>'user_key')::text = ? AND (data->>'cmd_id')::bigint = ? AND processed = false",
				ctx.GS.ID, stringedKey, ccID)).DeleteAll(context.Background(), common.PQ)
		if err != nil {
			return "", err
		}

		return "", nil
	}
}

type Query struct {
	UserID  null.Int64  `json:"user_id"`
	Pattern null.String `json:"pattern"`
	Reverse bool        `json:"reverse"`
}

func tmplDBSet(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}, value interface{}) (string, error) {
		return (tmplDBSetExpire(ctx))(userID, key, value, -1)
	}
}

func tmplDBSetExpire(ctx *templates.Context) func(userID int64, key interface{}, value interface{}, ttl int) (string, error) {
	return func(userID int64, key interface{}, value interface{}, ttl int) (string, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if aboveLimit, err := CheckGuildDBLimit(ctx.GS); err != nil || aboveLimit {
			if err != nil {
				return "", err
			}

			return "", errors.New("Above DB Limit")
		}

		valueSerialized, err := serializeValue(value)
		if err != nil {
			return "", err
		}

		vNum := templates.ToFloat64(value)
		keyStr := limitString(templates.ToString(key), 256)

		var expires null.Time
		if ttl > 0 {
			expires.Time = time.Now().Add(time.Second * time.Duration(ttl))
			expires.Valid = true
		}

		m := &models.TemplatesUserDatabase{
			GuildID:   ctx.GS.ID,
			UserID:    userID,
			UpdatedAt: time.Now(),
			ExpiresAt: expires,

			Key:      keyStr,
			ValueRaw: valueSerialized,
			ValueNum: vNum,
		}

		err = m.Upsert(context.Background(), common.PQ, true, []string{"guild_id", "user_id", "key"}, boil.Whitelist("value_raw", "value_num", "updated_at", "expires_at"), boil.Infer())
		return "", err
	}
}

func tmplDBIncr(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}, incrBy interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if aboveLimit, err := CheckGuildDBLimit(ctx.GS); err != nil || aboveLimit {
			if err != nil {
				return "", err
			}

			return "", errors.New("Above DB Limit")
		}

		vNum := templates.ToFloat64(incrBy)
		valueSerialized, err := serializeValue(vNum)
		if err != nil {
			return "", err
		}

		keyStr := limitString(templates.ToString(key), 256)

		const q = `INSERT INTO templates_user_database (created_at, updated_at, guild_id, user_id, key, value_raw, value_num)
VALUES (now(), now(), $1, $2, $3, $4, $5)
ON CONFLICT (guild_id, user_id, key)
DO UPDATE SET
	value_num =
		-- Don't increment expired entry
		CASE WHEN (templates_user_database.expires_at IS NULL OR templates_user_database.expires_at > now()) THEN templates_user_database.value_num + $5
		ELSE $5
		END,
	updated_at = now(),
	created_at =
		-- Reset created_at if the entry expired
		CASE WHEN (templates_user_database.expires_at IS NULL OR templates_user_database.expires_at > now()) THEN templates_user_database.created_at
		ELSE now()
		END,
	expires_at =
		-- Same for expires_at
		CASE WHEN (templates_user_database.expires_at IS NULL OR templates_user_database.expires_at > now()) THEN templates_user_database.expires_at
		ELSE NULL
		END

RETURNING value_num`

		result := common.PQ.QueryRow(q, ctx.GS.ID, userID, keyStr, valueSerialized, vNum)

		var newVal float64
		err = result.Scan(&newVal)
		return newVal, err
	}
}

func tmplDBGet(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		keyStr := limitString(templates.ToString(key), 256)
		m, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND user_id = ? AND key = ? AND (expires_at IS NULL OR expires_at > now())", ctx.GS.ID, userID, keyStr)).OneG(context.Background())
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}

			return nil, nil
		}

		return ToLightDBEntry(m)
	}
}

func tmplDBGetPattern(ctx *templates.Context, inverse bool) interface{} {
	order := "id asc"
	if inverse {
		order = "id desc"
	}

	return func(userID int64, pattern interface{}, iAmount interface{}, iSkip interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if ctx.IncreaseCheckCallCounterPremium("db_multiple", 2, 10) {
			return "", templates.ErrTooManyCalls
		}

		amount := int(templates.ToInt64(iAmount))
		skip := int(templates.ToInt64(iSkip))
		// LIMIT 0 essentially means LIMIT ALL, or no limit at all.
		// Make sure we actually cap it at the max documented limit.
		if amount > 100 || amount <= 0 {
			amount = 100
		}

		keyStr := limitString(templates.ToString(pattern), 256)
		results, err := models.TemplatesUserDatabases(
			qm.Where("guild_id = ? AND user_id = ? AND key LIKE ? AND (expires_at IS NULL OR expires_at > now())", ctx.GS.ID, userID, keyStr),
			qm.OrderBy(order), qm.Limit(amount), qm.Offset(skip)).AllG(context.Background())
		if err != nil {
			return nil, err
		}

		return tmplResultSetToLightDBEntries(ctx, ctx.GS, results), nil
	}
}

func tmplDBDel(ctx *templates.Context) interface{} {
	return func(userID int64, key interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		cachedDBLimits.Delete(ctx.GS.ID)

		keyStr := limitString(templates.ToString(key), 256)
		_, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND user_id = ? AND key = ?", ctx.GS.ID, userID, keyStr)).DeleteAll(context.Background(), common.PQ)

		return "", err
	}
}

func tmplDBDelById(ctx *templates.Context) interface{} {
	return func(userID int64, id int64) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		cachedDBLimits.Delete(ctx.GS.ID)

		_, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND user_id = ? AND id = ?", ctx.GS.ID, userID, id)).DeleteAll(context.Background(), common.PQ)

		return "", err
	}
}
func tmplDBDelMultiple(ctx *templates.Context) interface{} {
	return func(query interface{}, iAmount interface{}, iSkip interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if ctx.IncreaseCheckCallCounterPremium("db_multiple", 2, 10) {
			return "", templates.ErrTooManyCalls
		}

		q, err := queryFromArg(query)
		if err != nil {
			return "", err
		}

		amount := int(templates.ToInt64(iAmount))
		if amount > 100 {
			amount = 100
		}
		// LIMIT 0 essentially means LIMIT ALL, or no limit at all.
		// Make sure we actually cap it at the max documented limit.
		if amount == 0 {
			amount = 100
		}

		skip := int(templates.ToInt64(iSkip))
		orderby := "value_num DESC, id DESC"
		if q.Reverse {
			orderby = "value_num ASC, id ASC"
		}

		qms := []qm.QueryMod{qm.Where("guild_id = ?", ctx.GS.ID), qm.OrderBy(orderby), qm.Limit(amount), qm.Offset(skip)}
		if q.Pattern.Valid {
			qms = append(qms, qm.Where("key LIKE ?", limitString(q.Pattern.String, 256)))
		}
		if q.UserID.Valid {
			qms = append(qms, qm.Where("user_id = ?", q.UserID.Int64))
		}
		rows, err := models.TemplatesUserDatabases(qms...).AllG(context.Background())
		if err != nil {
			return "", err
		}

		cleared, err := rows.DeleteAllG(context.Background())
		cachedDBLimits.Delete(ctx.GS.ID)
		return cleared, err
	}
}

func tmplDBRank(ctx *templates.Context) interface{} {
	return func(query interface{}, userID int64, key string) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if ctx.IncreaseCheckCallCounterPremium("db_multiple", 2, 10) {
			return "", templates.ErrTooManyCalls
		}

		q, err := queryFromArg(query)
		if err != nil {
			return "", err
		}

		order := `DESC`
		if q.Reverse {
			order = `ASC`
		}

		if q.UserID.Valid && (q.UserID.Int64 != userID) { // some optimization
			return 0, nil
		}

		const rawquery = `SELECT position FROM
(
	SELECT user_id, key,
	RANK() OVER
	(
		ORDER BY 
		CASE WHEN $1 = 'ASC'  THEN value_num ELSE 0 END ASC,  
		CASE WHEN $1 = 'DESC' THEN value_num ELSE 0 END DESC,
		CASE WHEN $1 = 'ASC'  THEN id ELSE 0 END ASC,
		CASE WHEN $1 = 'DESC'  THEN id ELSE 0 END DESC
	) AS position
FROM templates_user_database WHERE (guild_id = $2) AND ($3::bigint IS NULL OR user_id = $3) AND ($4::text IS NULL OR key LIKE $4) AND (expires_at IS NULL OR expires_at > now())
) AS w
WHERE user_id = $5 AND key = $6`

		var rank int64
		err = common.PQ.QueryRow(rawquery, order, ctx.GS.ID, q.UserID, q.Pattern, userID, key).Scan(&rank)
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return rank, err

	}
}

func tmplDBCount(ctx *templates.Context) interface{} {
	return func(variadicArg ...interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if ctx.IncreaseCheckCallCounterPremium("db_multiple", 2, 10) {
			return "", templates.ErrTooManyCalls
		}

		var userID null.Int64
		var pattern null.String
		if len(variadicArg) > 0 {

			switch arg := variadicArg[0].(type) {
			case int64:
				userID.Int64 = arg
				userID.Valid = true
			case int:
				userID.Int64 = int64(arg)
				userID.Valid = true
			case string:
				patternStr := limitString(arg, 256)
				pattern.String = patternStr
				pattern.Valid = true
			default:
				q, err := queryFromArg(arg)
				if err != nil {
					return "", err
				}
				userID = q.UserID
				pattern = q.Pattern

			}

		}

		const q = `SELECT count(*) FROM templates_user_database WHERE (guild_id = $1) AND ($2::bigint IS NULL OR user_id = $2) AND ($3::text IS NULL OR key LIKE $3) AND (expires_at IS NULL or expires_at > now())`

		var count int64
		err := common.PQ.QueryRow(q, ctx.GS.ID, userID, pattern).Scan(&count)
		return count, err
	}
}

func queryFromArg(query interface{}) (*Query, error) {

	querySdict, err := templates.StringKeyDictionary(query)
	if err != nil {
		return nil, err
	}

	var q Query
	for key, val := range querySdict {
		switch key {
		case "userID":
			switch val.(type) {
			case int, int64:
				q.UserID.Int64 = templates.ToInt64(val)
				q.UserID.Valid = true

			default:
				return &q, errors.New("Invalid UserID datatype in query. Must be a number")
			}

		case "pattern":
			q.Pattern.String = limitString(templates.ToString(val), 256)
			q.Pattern.Valid = true

		case "reverse":
			revFlag, ok := val.(bool)
			if !ok {
				return &q, errors.New("Invalid reverse flag datatype in query. Must be a boolean value.")
			}
			q.Reverse = revFlag

		default:
			return &q, errors.New("Invalid Key: " + key + " passed to query constructor")
		}
	}

	return &q, nil
}

func tmplDBTopEntries(ctx *templates.Context, bottom bool) interface{} {
	orderBy := "value_num DESC, id DESC"
	if bottom {
		orderBy = "value_num ASC, id ASC"
	}

	return func(pattern interface{}, iAmount interface{}, iSkip interface{}) (interface{}, error) {
		if ctx.IncreaseCheckCallCounterPremium("db_interactions", 10, 50) {
			return "", templates.ErrTooManyCalls
		}

		if ctx.IncreaseCheckCallCounterPremium("db_multiple", 2, 10) {
			return "", templates.ErrTooManyCalls
		}

		amount := int(templates.ToInt64(iAmount))
		skip := int(templates.ToInt64(iSkip))
		// LIMIT 0 essentially means LIMIT ALL, or no limit at all.
		// Make sure we actually cap it at the max documented limit.
		if amount > 100 || amount <= 0 {
			amount = 100
		}

		keyStr := limitString(templates.ToString(pattern), 256)
		results, err := models.TemplatesUserDatabases(
			qm.Where("guild_id = ? AND key LIKE ? AND (expires_at IS NULL OR expires_at > now())", ctx.GS.ID, keyStr),
			qm.OrderBy(orderBy), qm.Limit(amount), qm.Offset(skip)).AllG(context.Background())
		if err != nil {
			return nil, err
		}

		return tmplResultSetToLightDBEntries(ctx, ctx.GS, results), nil
	}
}

func serializeValue(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := msgpack.NewEncoder(templates.LimitWriter(&b, 100000))
	err := enc.Encode(v)
	return b.Bytes(), err
}

// returns true if were above db limit for the specified guild
func CheckGuildDBLimit(gs *dstate.GuildSet) (bool, error) {
	limitMuliplier := 1
	if isPremium, _ := premium.IsGuildPremium(gs.ID); isPremium {
		limitMuliplier = 10
	}

	limit := gs.MemberCount * 50 * int64(limitMuliplier)

	curValues, err := cacheCheckDBLimit(gs)
	if err != nil {
		return false, err
	}

	return curValues >= int64(limit), nil
}

func getGuildCCDBNumValues(guildID int64) (int64, error) {
	count, err := models.TemplatesUserDatabases(qm.Where("guild_id = ? AND (expires_at > now() OR expires_at IS NULL)", guildID)).CountG(context.Background())
	return count, err
}

var cachedDBLimits = common.CacheSet.RegisterSlot("custom_commands_db_limits", nil, int64(0))

func cacheCheckDBLimit(gs *dstate.GuildSet) (int64, error) {
	v, err := cachedDBLimits.GetCustomFetch(gs.ID, func(key interface{}) (interface{}, error) {
		n, err := getGuildCCDBNumValues(gs.ID)
		return n, err
	})

	if err != nil {
		return 0, err
	}

	return v.(int64), nil
}

// limitstring cuts off a string at max l length, supports multi byte characters
func limitString(s string, l int) string {
	if len(s) <= l {
		return s
	}

	lastValidLoc := 0
	for i := range s {
		if i > l {
			break
		}
		lastValidLoc = i
	}

	return s[:lastValidLoc]
}

type LightDBEntry struct {
	ID      int64
	GuildID int64
	UserID  int64

	CreatedAt time.Time
	UpdatedAt time.Time

	Key   string
	Value interface{}

	ValueSize int

	User discordgo.User

	ExpiresAt time.Time
}

func ToLightDBEntry(m *models.TemplatesUserDatabase) (*LightDBEntry, error) {
	var dst interface{}
	dec := newDecoder(bytes.NewBuffer(m.ValueRaw))
	err := dec.Decode(&dst)
	if err != nil {
		return nil, err
	}

	decodedValue := dst
	if common.IsNumber(dst) {
		decodedValue = m.ValueNum
	}

	entry := &LightDBEntry{
		ID:      m.ID,
		GuildID: m.GuildID,
		UserID:  m.UserID,

		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		Key:   m.Key,
		Value: decodedValue,

		ValueSize: len(m.ValueRaw),

		ExpiresAt: m.ExpiresAt.Time,
	}
	entry.User.ID = entry.UserID

	return entry, nil
}

func newDecoder(buf *bytes.Buffer) *msgpack.Decoder {
	dec := msgpack.NewDecoder(buf)

	dec.SetDecodeMapFunc(func(d *msgpack.Decoder) (interface{}, error) {
		n, err := d.DecodeMapLen()
		if err != nil {
			return nil, err
		}

		isStringKeysOnly := true
		mi := make(map[interface{}]interface{}, n)
		ms := make(map[string]interface{})

		for i := 0; i < n; i++ {
			mk, err := d.DecodeInterface()
			if err != nil {
				return nil, err
			}

			mv, err := d.DecodeInterface()
			if err != nil {
				return nil, err
			}

			// if the map only has string keys, use a map[string]interface{}
			if isStringKeysOnly {
				if s, ok := mk.(string); ok {
					// so far only string keys
					ms[s] = mv
				} else {
					// copy over the map to the interface{} keyed one
					isStringKeysOnly = false
					for jk, jv := range ms {
						mi[jk] = jv
					}
					mi[mk] = mv
				}
			} else {
				mi[mk] = mv
			}
		}
		if isStringKeysOnly {
			return ms, nil
		}

		return mi, nil
	})

	return dec
}

func tmplResultSetToLightDBEntries(ctx *templates.Context, gs *dstate.GuildSet, rs []*models.TemplatesUserDatabase) []*LightDBEntry {
	// convert them into lightdb entries and decode their values
	entries := make([]*LightDBEntry, 0, len(rs))
	for _, v := range rs {
		converted, err := ToLightDBEntry(v)
		if err != nil {
			ctx.LogEntry().WithError(err).Error("[cc] failed decoding user db entry")
			continue
		}

		entries = append(entries, converted)
	}

	// fill in user fields
	membersToFetch := make([]int64, 0, len(entries))
	for _, v := range entries {
		if common.ContainsInt64Slice(membersToFetch, v.UserID) {
			continue
		}

		membersToFetch = append(membersToFetch, v.UserID)
	}

	// fast path in case of single member
	if len(membersToFetch) == 1 {
		member, err := bot.GetMember(gs.ID, membersToFetch[0])
		if err != nil {
			ctx.LogEntry().WithError(err).Error("[cc] failed retrieving member")
			return entries
		}

		for _, v := range entries {
			v.User = member.User
		}

		return entries
	}

	// multiple members
	members, err := bot.GetMembers(gs.ID, membersToFetch...)
	if err != nil {
		ctx.LogEntry().WithError(err).Error("[cc] failed bot.GetMembers call")
	}

	for _, v := range entries {
		for _, m := range members {
			if m.User.ID == v.UserID {
				v.User = m.User
				break
			}
		}
	}

	return entries
}
