package roblox

import (
	"github.com/RhykerWells/yagpdb/v2/common"
	"github.com/RhykerWells/robloxgo"
	"github.com/RhykerWells/yagpdb/v2/common/config"
)

//go:generate sqlboiler --no-hooks --add-soft-deletes psql

var (
	RobloxClient *robloxgo.Client
	clientAPIKey = config.RegisterOption("yagpdb.robloxapikey", "the roblox api key used to manage and make requests", nil)
)

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)
	if clientAPIKey.GetString() != "" {
		RobloxClient, _ = robloxgo.Create(clientAPIKey.GetString())
	}
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Roblox",
		SysName:  "roblox",
		Category: common.PluginCategoryMisc,
	}
}
