package youtube

import (
	"context"
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/youtube/models"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func (p *Plugin) Status() (string, string) {
	var unique int
	common.RedisPool.Do(radix.Cmd(&unique, "ZCARD", RedisKeyWebSubChannels))

	total, _ := models.YoutubeChannelSubscriptions().CountG(context.Background())

	return "Unique/Total", fmt.Sprintf("%d/%d", unique, total)
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	toDisable, err := models.YoutubeChannelSubscriptions(
		models.YoutubeChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(guildID)),
		models.YoutubeChannelSubscriptionWhere.Enabled.EQ(true),

		qm.Offset(GuildMaxEnabledFeeds),
		qm.OrderBy("id DESC"),
	).AllG(context.Background())

	for _, f := range toDisable {
		f.Enabled = false
		f.UpdateG(context.Background(), boil.Infer())
	}

	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed disabling excess feeds")
		return err
	}

	logger.WithField("guild", guildID).Infof("disabled %d excess feeds", len(toDisable))
	return nil
}
