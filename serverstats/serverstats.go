package serverstats

//go:generate sqlboiler --no-hooks psql

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/serverstats/models"
)

var confDeprecated = config.RegisterOption("yagpdb.serverstats.deprecated", "Wether to mark server stats as disabled or not, this will disable recording of new stats", false)

type Plugin struct {
	stopStatsLoop chan *sync.WaitGroup
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Server Stats",
		SysName:  "server_stats",
		Category: common.PluginCategoryMisc,
	}
}

var logger = common.GetPluginLogger(&Plugin{})

func RegisterPlugin() {
	common.InitSchemas("serverstats", dbSchemas...)

	plugin := &Plugin{
		stopStatsLoop: make(chan *sync.WaitGroup),
	}
	common.RegisterPlugin(plugin)
}

// ServerStatsConfig represents a configuration for a server
// reason we dont reference the model directly is because i need to figure out a way to
// migrate them over to the new schema, painlessly.
type ServerStatsConfig struct {
	Public         bool
	IgnoreChannels string

	ParsedChannels []int64
}

func (s *ServerStatsConfig) ParseChannels() {
	split := strings.Split(s.IgnoreChannels, ",")
	for _, v := range split {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			s.ParsedChannels = append(s.ParsedChannels, parsed)
		}
	}
}

func configFromModel(model *models.ServerStatsConfig) *ServerStatsConfig {
	conf := &ServerStatsConfig{
		Public:         model.Public.Bool,
		IgnoreChannels: model.IgnoreChannels.String,
	}
	conf.ParseChannels()

	return conf
}

func GetConfig(ctx context.Context, GuildID int64) (*ServerStatsConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	conf, err := models.FindServerStatsConfigG(ctx, GuildID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if conf == nil {
		return &ServerStatsConfig{}, nil
	}

	return configFromModel(conf), nil
}

// RoundHour rounds a time.Time down to the hour
func RoundHour(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

var _ premium.NewPremiumGuildListener = (*Plugin)(nil)

func (p *Plugin) OnNewPremiumGuild(guildID int64) error {

	const q = `UPDATE server_stats_periods_compressed SET premium=true WHERE guild_id=$1 AND premium=false`

	_, err := common.PQ.Exec(q, guildID)
	return err
}
