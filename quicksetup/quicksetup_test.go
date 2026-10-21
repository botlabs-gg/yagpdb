package quicksetup

import (
	"slices"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/premium"
)

type testPlugin struct{ sysName string }

func (p *testPlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{SysName: p.sysName}
}

func TestAvailableModulesFollowLoadedPlugins(t *testing.T) {
	defer func(saved []common.Plugin) { common.Plugins = saved }(common.Plugins)
	common.Plugins = []common.Plugin{&testPlugin{"moderation"}, &testPlugin{"autorole"}}

	var ids []string
	for _, m := range availableModules() {
		ids = append(ids, m.ID)
	}

	if !slices.Equal(ids, []string{"moderation", "autorole"}) {
		t.Errorf("got %v", ids)
	}
	if moduleAvailable("verification") {
		t.Error("verification offered without its plugin")
	}
}

func TestMissingPermNames(t *testing.T) {
	required := []int64{discordgo.PermissionManageRoles, discordgo.PermissionBanMembers}

	if missing := missingPermNames(discordgo.PermissionBanMembers, required); len(missing) != 1 || missing[0] != common.StringPerms[discordgo.PermissionManageRoles] {
		t.Errorf("got %v", missing)
	}
	if missing := missingPermNames(discordgo.PermissionAdministrator, required); missing != nil {
		t.Errorf("administrator still missing %v", missing)
	}
}

func TestShowcasesLeaveOutDisabledPlugins(t *testing.T) {
	defer func(saved []common.Plugin) { common.Plugins = saved }(common.Plugins)
	common.Plugins = []common.Plugin{&testPlugin{"custom_commands"}, &testPlugin{"rss"}}

	var names []string
	for _, feature := range loadedFeatures(suggestedFeatures) {
		names = append(names, feature.Name)
	}
	if !slices.Equal(names, []string{"Custom Commands", "RSS Feeds"}) {
		t.Errorf("suggested %v with only custom commands and rss loaded", names)
	}

	for _, perk := range premium.AvailablePerks() {
		if perk.Path == "/twitch" {
			t.Error("twitch perk shown with twitch disabled")
		}
	}
}
