package logs

import (
	"context"
	"time"

	"emperror.dev/errors"
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

func tmplUsernames(tmplCtx *templates.Context) interface{} {
	return func(userIDi interface{}, offset int) (interface{}, error) {
		if tmplCtx.IncreaseCheckCallCounter("pastUsernames", 2) {
			return nil, errors.New("Max calls to pastUsernames (2) reached")
		}

		target := templates.ToInt64(userIDi)
		result := make([]*CCNameChange, 0)

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

func tmplNicknames(tmplCtx *templates.Context) interface{} {
	return func(userIDi interface{}, offset int) (interface{}, error) {
		if tmplCtx.IncreaseCheckCallCounter("pastNicknames", 2) {
			return nil, errors.New("Max calls to pastNicknames (2) reached")
		}

		target := templates.ToInt64(userIDi)
		result := make([]*CCNameChange, 0)

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
