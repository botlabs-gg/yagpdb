// Package quicksetup walks a server through setting up the main features with
// sensible defaults, step by step, and can be rerun or skipped at any time.
package quicksetup

import (
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/mediocregopher/radix/v3"
)

type Plugin struct{}

func RegisterPlugin() {
	common.RegisterPlugin(&Plugin{})
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Quick Setup",
		SysName:  "quick_setup",
		Category: common.PluginCategoryCore,
	}
}

type setupState string

const (
	setupStateNone      setupState = ""
	setupStateCompleted setupState = "completed"
	setupStateSkipped   setupState = "skipped"
)

func stateKey(guildID int64) string {
	return "quicksetup:" + strconv.FormatInt(guildID, 10) + ":state"
}

func getSetupState(guildID int64) (setupState, error) {
	var state string
	err := common.RedisPool.Do(radix.Cmd(&state, "GET", stateKey(guildID)))
	return setupState(state), err
}

func setSetupState(guildID int64, state setupState) error {
	return common.RedisPool.Do(radix.Cmd(nil, "SET", stateKey(guildID), string(state)))
}
