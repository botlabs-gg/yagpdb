package premium

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

type testPlugin struct{ sysName string }

func (p *testPlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{SysName: p.sysName}
}

func TestPerksAndLimitsFollowLoadedPlugins(t *testing.T) {
	defer func(saved []common.Plugin) { common.Plugins = saved }(common.Plugins)
	common.Plugins = []common.Plugin{&testPlugin{"tickets"}, &testPlugin{"rss"}}

	if available := AvailablePerks(); len(available) != 1 || available[0].Name != "Threaded Tickets" {
		t.Errorf("got perks %+v", available)
	}

	var names []string
	for _, limit := range AvailableLimits() {
		names = append(names, limit.Name)
	}
	if len(names) != 2 || names[0] != "Modlog cache" || names[1] != "RSS feeds" {
		t.Errorf("got limits %v, want the modlog cache, which every server has, and rss", names)
	}
}
