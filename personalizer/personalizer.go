package personalizer

import (
	"context"
	"database/sql"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/personalizer/models"
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Personalizer",
		SysName:  "personalizer",
		Category: common.PluginCategoryCore,
	}
}

func RegisterPlugin() {
	common.InitSchemas("personalizer", DBSchemas...)
	common.RegisterPlugin(&Plugin{})
}

// OnRemovedPremiumGuild clears saved personalization for the guild and resets member avatar/banner
func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	ctx := context.Background()
	pg, err := models.FindPersonalizedGuildG(ctx, guildID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			pg = nil
		} else {
			return err
		}
	}

	if pg != nil {
		_, _ = pg.DeleteG(ctx)
	}

	err = common.BotSession.GuildMemberMeReset(guildID, true, true, false)
	return err
}
