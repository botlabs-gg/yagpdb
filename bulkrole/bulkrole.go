package bulkrole

import (
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var configCache = common.CacheSet.RegisterSlot("bulkrole_config", func(key interface{}) (interface{}, error) {
	config, err := GetBulkRoleConfig(key.(int64))
	return config, err
}, int64(0))

var logger = common.GetPluginLogger(&Plugin{})

func KeyGeneral(guildID int64) string { return "bulkrole:" + discordgo.StrID(guildID) + ":general" }
func KeyProcessing(guildID int64) string {
	return "bulkrole:" + discordgo.StrID(guildID) + ":processing"
}

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Bulk Role Manager",
		SysName:  "bulkrole",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
}

type BulkRoleConfig struct {
	TargetRole int64 `json:",string" valid:"role,true"`

	Operation string `valid:"in(assign|remove)"`

	FilterType string `valid:"in(all|has_role|missing_role|bots|humans|joined_after|joined_before)"`

	FilterRoleIDs    []int64   `json:",omitempty"`
	FilterRequireAll bool      `json:"boolean,omitempty"`
	FilterDate       string    `json:",omitempty"`
	FilterDateParsed time.Time `json:"-"`

	NotificationChannel int64 `json:",string" valid:"channel,true"`

	StartedBy int64 `json:",string"`
}

const (
	BulkRoleStarted int = iota + 1
	BulkRoleIterating
	BulkRoleIterationDone
	BulkRoleProcessing
	BulkRoleCancelled
	BulkRoleCompleted
)

func GetBulkRoleConfig(guildID int64) (*BulkRoleConfig, error) {
	conf := &BulkRoleConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retrieving bulkrole config")
	}
	return conf, err
}
