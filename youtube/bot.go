package youtube

import (
	"fmt"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
)

func (p *Plugin) Status(client *redis.Client) (string, string) {
	numUnique, _ := client.Cmd("ZCARD", "youtube_subbed_channels").Int()

	var numChannels int
	common.GORM.Model(&ChannelSubscription{}).Count(&numChannels)

	return "Youtube", fmt.Sprintf("%d/%d", numUnique, numChannels)
}
