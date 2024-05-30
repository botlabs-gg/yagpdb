package youtube

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/mediocregopher/radix/v3"
)

func (p *Plugin) Status() (string, string) {
	var unique int
	common.RedisPool.Do(radix.Cmd(&unique, "ZCARD", RedisKeyWebSubChannels))

	var numChannels int
	common.GORM.Model(&ChannelSubscription{}).Count(&numChannels)

	return "Unique/Total", fmt.Sprintf("%d/%d", unique, numChannels)
}

func (p *Plugin) OnRemovedPremiumGuild(guildID int64) error {
	logger.WithField("guild_id", guildID).Infof("Removed Excess Youtube Feeds")
	feeds := make([]ChannelSubscription, 0)
	err := common.GORM.Model(&ChannelSubscription{}).Where(`guild_id = ? and enabled = ?`, guildID, common.BoolToPointer(true)).Offset(GuildMaxFeeds).Order(
		"id desc",
	).Find(&feeds).Error
	if err != nil {
		logger.WithError(err).Errorf("failed getting feed ids for guild_id %d", guildID)
		return err
	}

	if len(feeds) > 0 {
		err = common.GORM.Model(&feeds).Update(ChannelSubscription{Enabled: common.BoolToPointer(false)}).Error
		if err != nil {
			logger.WithError(err).Errorf("failed getting feed ids for guild_id %d", guildID)
			return err
		}
	}
	return nil
}
