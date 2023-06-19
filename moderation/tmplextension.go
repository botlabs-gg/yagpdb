package moderation

import (
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/templates"
	"github.com/jinzhu/gorm"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		ctx.ContextFuncs["getWarns"] = tmplGetWarns(ctx)
	})
}

// getWarns returns a slice of all warnings the target user has.
func tmplGetWarns(ctx *templates.Context) interface{} {
	return func(target interface{}) ([]*WarningModel, error) {
		if ctx.IncreaseCheckGenericAPICall() {
			return nil, nil
		}

		gID := ctx.GS.ID
		var warns []*WarningModel
		targetID := templates.TargetUserID(target)

		err := common.GORM.Where("user_id = ? AND guild_id = ?", targetID, gID).Order("id desc").Find(&warns).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}

		return warns, nil
	}
}
