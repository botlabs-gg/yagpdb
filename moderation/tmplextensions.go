package moderation

import (
	"fmt"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/botlabs-gg/yagpdb/v2/logs"
	"github.com/jinzhu/gorm"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["getWarnings"] = tmplGetWarnings(ctx)
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
			return nil, fmt.Errorf("could not convert %T to a user ID", target)
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
