package logs

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["pastUsernames"] = tmplUsernames(ctx)
		ctx.ContextFuncs["pastNicknames"] = tmplNicknames(ctx)
	})
}

type CCNameChange struct {
	Name string
	Time time.Time
}

func tmplUsernames(tmplCtx *templates.Context) any {
	return func(userIDi any, offset int) (any, error) {
		if !confEnableUsernameTracking.GetBool() {
			return nil, errors.New("pastUsernames has been disabled globally")
		}
		if tmplCtx.IncreaseCheckCallCounter("pastUsernames", 2) {
			return nil, errors.New("Max calls to pastUsernames (2) reached")
		}

		target := templates.ToInt64(userIDi)
		result := make([]*CCNameChange, 0)

		// username history is collected across all servers, so like the usernames command only
		// members of this server can be looked up, and only if it has username logging on
		config, err := GetConfig(common.PQ, context.Background(), tmplCtx.GS.ID)
		if err != nil {
			return nil, err
		}
		if !config.UsernameLoggingEnabled.Bool {
			return nil, errors.New("Username logging is disabled on this server")
		}
		if ms, err := bot.GetMember(tmplCtx.GS.ID, target); err != nil || ms == nil {
			return nil, errors.New("pastUsernames only works for members of this server")
		}

		usernames, err := GetUsernames(context.Background(), target, 15, offset)
		if err != nil {
			return nil, err
		}

		for _, v := range usernames {
			result = append(result, &CCNameChange{
				Name: v.Username.String,
				Time: v.CreatedAt.Time,
			})
		}

		return result, nil
	}
}

func tmplNicknames(tmplCtx *templates.Context) any {
	return func(userIDi any, offset int) (any, error) {
		if !confEnableUsernameTracking.GetBool() {
			return nil, errors.New("pastNicknames has been disabled globally")
		}
		if tmplCtx.IncreaseCheckCallCounter("pastNicknames", 2) {
			return nil, errors.New("Max calls to pastNicknames (2) reached")
		}

		target := templates.ToInt64(userIDi)
		result := make([]*CCNameChange, 0)

		config, err := GetConfig(common.PQ, context.Background(), tmplCtx.GS.ID)
		if err != nil {
			return nil, err
		}
		if !config.NicknameLoggingEnabled.Bool {
			return nil, errors.New("Nickname logging is disabled on this server")
		}

		nicknames, err := GetNicknames(context.Background(), target, tmplCtx.GS.ID, 15, offset)
		if err != nil {
			return nil, err
		}

		for _, v := range nicknames {
			result = append(result, &CCNameChange{
				Name: v.Nickname.String,
				Time: v.CreatedAt.Time,
			})
		}

		return result, nil
	}
}
