package moderation

import (
	"errors"
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/logs"
	"github.com/jinzhu/gorm"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["getWarnings"] = tmplGetWarnings(ctx)
		ctx.ContextFuncs["muteUser"] = tmplMuteUser(ctx)
		ctx.ContextFuncs["unmuteUser"] = tmplUnmuteUser(ctx)
	})
}

// getWarnings returns a slice of all warnings the target user has.
func tmplGetWarnings(ctx *templates.Context) interface{} {
	return func(target interface{}) ([]*WarningModel, error) {
		if ctx.IncreaseCheckCallCounterPremium("cc_moderation", 5, 10) {
			return nil, templates.ErrTooManyCalls
		}

		gID := ctx.GS.ID
		var warns []*WarningModel
		targetID := templates.TargetUserID(target)
		if targetID == 0 {
			return nil, fmt.Errorf("Could not convert %T to a user ID", target)
		}

		err := common.GORM.Where("user_id = ? AND guild_id = ?", targetID, gID).Order("id desc").Find(&warns).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}

		// Avoid listing expired logs.
		for _, entry := range warns {
			purgedWarnLogs := logs.ConfEnableMessageLogPurge.GetBool() && entry.CreatedAt.Before(time.Now().AddDate(0, 0, -30))
			if entry.LogsLink != "" && purgedWarnLogs {
				entry.LogsLink = ""
			}
		}

		return warns, nil
	}
}

// muteUser mutes the target user for the specified duration.
func tmplMuteUser(ctx *templates.Context) interface{} {
	return func(target interface{}, duration string, reason string) (string, error) {
		if ctx.IncreaseCheckCallCounterPremium("cc_moderation", 5, 10) {
			return "", templates.ErrTooManyCalls
		}

		config, err := GetConfig(ctx.GS.ID)
		if err != nil {
			return "", err
		}

		if config.MuteRole == "" {
			return "", errors.New("No mute role set up")
		}

		targetID := templates.TargetUserID(target)
		if targetID == 0 {
			return "", fmt.Errorf("Could not convert %T to a user ID", target)
		}

		member, err := bot.GetMember(ctx.GS.ID, targetID)
		if err != nil || member == nil {
			return "", errors.New("Could not find member")
		}

		dur, err := common.ParseDuration(duration)
		if err != nil {
			return "", err
		}

		err = MuteUnmuteUser(config, true, ctx.GS.ID, ctx.CurrentFrame.CS, ctx.Msg, ctx.Msg.Author, reason, member, int(dur.Minutes()))
		if err != nil {
			return "", err
		}

		return "", nil
	}
}

// unmuteUser unmutes the target user.
func tmplUnmuteUser(ctx *templates.Context) interface{} {
	return func(target interface{}, reason string) (string, error) {
		if ctx.IncreaseCheckCallCounterPremium("cc_moderation", 5, 10) {
			return "", templates.ErrTooManyCalls
		}

		config, err := GetConfig(ctx.GS.ID)
		if err != nil {
			return "", err
		}

		if config.MuteRole == "" {
			return "", errors.New("No mute role set up")
		}

		targetID := templates.TargetUserID(target)
		if targetID == 0 {
			return "", fmt.Errorf("Could not convert %T to a user ID", target)
		}

		member, err := bot.GetMember(ctx.GS.ID, targetID)
		if err != nil || member == nil {
			return "", errors.New("Could not find member")
		}

		err = MuteUnmuteUser(config, false, ctx.GS.ID, ctx.CurrentFrame.CS, ctx.Msg, ctx.Msg.Author, reason, member, 0)
		if err != nil {
			return "", err
		}

		return "", nil
	}
}
