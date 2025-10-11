package bulkrole

import (
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/mediocregopher/radix/v3"
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

	StartedBy         int64  `json:",string"`
	StartedByUsername string `json:",omitempty"`
	GuildID           int64  `json:",string"`
}

const (
	BulkRoleStarted int = iota + 1
	BulkRoleIterating
	BulkRoleIterationDone
	BulkRoleProcessing
	BulkRoleCancelled
	BulkRoleCompleted
)

type StatusResponse struct {
	Status    int `json:"status"`
	Processed int `json:"processed"`
	Results   int `json:"results"`
}

func GetBulkRoleConfig(guildID int64) (*BulkRoleConfig, error) {
	conf := &BulkRoleConfig{}
	err := common.GetRedisJson(KeyGeneral(guildID), conf)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).Error("failed retrieving bulkrole config")
	}
	// Recompute parsed fields that are not persisted
	if conf.FilterDate != "" {
		if parsed, perr := time.Parse("2006-01-02", conf.FilterDate); perr == nil {
			conf.FilterDateParsed = parsed
		}
	}
	conf.GuildID = guildID
	return conf, err
}

func IsBulkRoleOperationActive(guildID int64) bool {
	var status int
	common.RedisPool.Do(radix.Cmd(&status, "GET", RedisKeyBulkRoleStatus(guildID)))
	return status > 0
}
