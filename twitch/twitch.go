package twitch

import (
	"context"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/twitch/models"
	"github.com/nicklaw5/helix/v2"
)

//go:generate sqlboiler --no-hooks psql

var (
	confClientId     = config.RegisterOption("yagpdb.twitch.clientid", "Twitch Client ID", "")
	confClientSecret = config.RegisterOption("yagpdb.twitch.clientsecret", "Twitch Client Secret", "")
	logger           = common.GetPluginLogger(&Plugin{})
)

const (
	GuildMaxFeeds               = 15
	GuildMaxEnabledFeeds        = 3
	GuildMaxEnabledFeedsPremium = 15
)

func MaxFeedsForContext(ctx context.Context) int {
	if premium.ContextPremium(ctx) {
		return GuildMaxEnabledFeedsPremium
	}
	return GuildMaxEnabledFeeds
}

type Plugin struct {
	HelixClient *helix.Client
	Stop        chan *sync.WaitGroup
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Twitch",
		SysName:  "twitch",
		Category: common.PluginCategoryFeeds,
	}
}

func RegisterPlugin() {
	p := &Plugin{}

	mqueue.RegisterSource("twitch", p)

	err := p.SetupClient()
	if err != nil {
		logger.WithError(err).Error("Failed setting up twitch plugin, twitch plugin will not be enabled.")
		return
	}
	common.RegisterPlugin(p)

	common.InitSchemas("twitch", DBSchemas...)
}

var _ mqueue.PluginWithSourceDisabler = (*Plugin)(nil)

func (p *Plugin) DisableFeed(elem *mqueue.QueuedElement, err error) {
	p.DisableChannelFeeds(elem.ChannelID)
}

func (p *Plugin) DisableChannelFeeds(channelID int64) error {
	_, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.ChannelID.EQ(discordgo.StrID(channelID))).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("channel", channelID).Error("failed disabling feeds in channel")
	}
	return err
}

func (p *Plugin) DisableGuildFeeds(guildID int64) error {
	_, err := models.TwitchChannelSubscriptions(models.TwitchChannelSubscriptionWhere.GuildID.EQ(discordgo.StrID(guildID))).UpdateAllG(context.Background(), models.M{"enabled": false})
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed disabling feeds in guild")
	}
	return err
}
