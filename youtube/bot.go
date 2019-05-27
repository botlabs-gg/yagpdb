package youtube

import (
	"fmt"

	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/common"
)

func (p *Plugin) Status() (string, string) {
	var unique int
	common.RedisPool.Do(retryableredis.Cmd(&unique, "ZCARD", "youtube_subbed_channels"))

	var numChannels int
	common.GORM.Model(&ChannelSubscription{}).Count(&numChannels)

	return "Youtube", fmt.Sprintf("%d/%d", unique, numChannels)
}
