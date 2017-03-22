package youtube

import (
	"fmt"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/common"
)

func (p *Plugin) InitBot() {}

func (p *Plugin) Status(client *redis.Client) (string, string) {
	numUnique, _ := client.Cmd("ZCARD", "youtube_subbed_channels").Int()

	var numChannels int
	common.GORM.Model(&ChannelSubscription{}).Count(&numChannels)

	return "Youtube", fmt.Sprintf("%d/%d", numUnique, numChannels)
}
