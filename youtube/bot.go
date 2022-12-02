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

	return "Youtube", fmt.Sprintf("%d/%d", unique, numChannels)
}
