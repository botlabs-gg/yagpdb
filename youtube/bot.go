package youtube

import (
	"fmt"

	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
)

func (p *Plugin) Status() (string, string) {
	var unique int
	common.RedisPool.Do(radix.Cmd(&unique, "ZCARD", "youtube_subbed_channels"))

	var numChannels int
	common.GORM.Model(&ChannelSubscription{}).Count(&numChannels)

	return "Youtube", fmt.Sprintf("%d/%d", unique, numChannels)
}
