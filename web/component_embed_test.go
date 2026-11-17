package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

type fakePlugin struct {
	sysName string
	active  bool
}

func (p *fakePlugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{SysName: p.sysName}
}

func (p *fakePlugin) IsActive() bool { return p.active }

func withPlugins(t *testing.T, plugins ...common.Plugin) {
	t.Helper()
	old := common.Plugins
	common.Plugins = plugins
	t.Cleanup(func() { common.Plugins = old })
}

func TestLandingComponentEmbedFitsDiscordLimits(t *testing.T) {
	var all []common.Plugin
	for _, line := range landingFeatureLines {
		for _, f := range line.features {
			all = append(all, &fakePlugin{sysName: f.sysName, active: true})
		}
	}
	withPlugins(t, all...)

	encoded, err := landingComponentEmbed().Encode()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("landing component embed is %d bytes with every feature active", len(encoded))

	var decoded struct {
		Component map[string]any `json:"component"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Component["type"] != float64(17) {
		t.Errorf("top level component must be a container, got %v", decoded.Component["type"])
	}
}

func TestLandingFeatureListOnlyActivePlugins(t *testing.T) {
	withPlugins(t,
		&fakePlugin{sysName: "custom_commands", active: true},
		&fakePlugin{sysName: "youtube", active: true},
		&fakePlugin{sysName: "rss", active: true},
		&fakePlugin{sysName: "twitch", active: true},
		&fakePlugin{sysName: "server_stats", active: false},
		&fakePlugin{sysName: "tickets", active: true},
	)

	want := "### Features\n" +
		"- **Custom Commands** with a full templating language\n" +
		"- **Feeds**: YouTube, RSS and Twitch\n" +
		"- **Tickets** and more"
	if got := landingFeatureList(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLandingComponentEmbedWithoutFeatures(t *testing.T) {
	withPlugins(t)

	encoded, err := landingComponentEmbed().Encode()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Features") {
		t.Error("expected no feature list when no plugins are active")
	}
}
