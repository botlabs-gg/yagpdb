package featureflags

import (
	"fmt"
	"os"
	"testing"

	"github.com/jonas747/yagpdb/common"
)

func TestMain(m *testing.M) {
	if err := common.InitTestRedis(); err != nil {
		fmt.Printf("Failed redis init, not running tests... %v \n", err)
		return
	}

	os.Exit(m.Run())
}

type FakePlugin struct {
	ActiveFlags []string
	AllFlags    []string
}

func (f *FakePlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "test_plugin",
		SysName:  "test_plugin",
		Category: common.PluginCategoryCore,
	}
}

func (f *FakePlugin) UpdateFeatureFlags(guildID int64) ([]string, error) {
	return f.ActiveFlags, nil
}
func (f *FakePlugin) AllFeatureFlags() []string {
	return f.AllFlags
}

func (f *FakePlugin) checkFlags(t *testing.T, flags []string) {
	for _, v := range f.ActiveFlags {
		has := common.ContainsStringSlice(flags, v)
		if v == "unknown" {
			if has {
				t.Error("'unknown' flag was set")
			}
		} else if !has {
			t.Errorf("missing flag %q", v)
		}
	}
}

func TestFeatureFlags(t *testing.T) {
	p := &FakePlugin{
		ActiveFlags: []string{"epic", "feature", "unknown"},
		AllFlags:    []string{"epic", "feature", "disabled", "something_else"},
	}

	err := UpdatePluginFeatureFlags(1, p)
	if err != nil {
		t.Errorf("Error on updating flags %v", err)
	}

	flags, err := GetGuildFlags(1)
	if err != nil {
		t.Errorf("Error on retrieving flags %v", err)
	}

	p.checkFlags(t, flags)

	p.ActiveFlags = []string{"epic"}
	err = UpdatePluginFeatureFlags(1, p)
	if err != nil {
		t.Errorf("Error on updating flags %v", err)
	}

	invalidateCache(1)

	flags, err = GetGuildFlags(1)
	if err != nil {
		t.Errorf("Error on retrieving flags %v", err)
	}

	p.checkFlags(t, flags)

	has, err := GuildHasFlag(1, "epic")
	if err != nil {
		t.Errorf("Error on checking flags %v", err)
	}

	if !has {
		t.Error("flag 'epic' not set")
	}
}

func invalidateCache(guildID int64) {
	cacheID := (guildID >> 22) % int64(len(caches))
	caches[cacheID].invalidateGuild(guildID)
}
