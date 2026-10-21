package premium

import "github.com/botlabs-gg/yagpdb/v2/common"

// Perk is a feature only premium servers get. Path is its page on a
// server's dashboard.
type Perk struct {
	Name        string
	Description string
	Icon        string
	Path        string

	pluginSysName string
}

// Limit is a limit premium raises. An empty plugin applies to every server.
type Limit struct {
	Name string
	Icon string
	From string
	To   string

	pluginSysName string
}

var perks = []*Perk{
	{"Custom Bot Avatar & Banner", "Give YAGPDB its own face and banner in your server.", "fas fa-image", "/personalizer", "personalizer"},
	{"Trigger CCs on Message Edit", "Re-run custom commands when a user edits their message.", "fas fa-edit", "/customcommands", "custom_commands"},
	{"Bulk Role Assignment", "Add or remove roles for many members in one go.", "fas fa-users-cog", "/bulkrole", "bulkrole"},
	{"Custom Thanks Regex", "Define how reputation's thanks detection matches messages.", "fas fa-heart", "/reputation", "reputation"},
	{"Custom Twitch Announcements", "Replace the default go-live message with your own.", "fab fa-twitch", "/twitch", "twitch"},
	{"Retroactive Autorole Scan", "Apply your autorole to existing members, not just new joins.", "fas fa-user-plus", "/autorole", "autorole"},
	{"Threaded Tickets", "Use private threads instead of new channels for tickets.", "fas fa-comments", "/tickets", "tickets"},
	{"Appeal Ban Button", "Let banned users appeal straight from their ban DM.", "fas fa-gavel", "/moderation", "moderation"},
	{"Ban Appeals Channel", "Post ban appeals in their own channel instead of the modlog.", "fas fa-inbox", "/moderation", "moderation"},
}

var limits = []*Limit{
	{"Modlog cache", "fas fa-history", "1h", "12h", ""},
	{"Custom commands", "fas fa-terminal", "100", "500", "custom_commands"},
	{"Reddit feeds", "fab fa-reddit", "20", "1000", "reddit"},
	{"YouTube feeds", "fab fa-youtube", "10", "250", "youtube"},
	{"Twitch feeds", "fab fa-twitch", "3", "15", "twitch"},
	{"Soundboard", "fas fa-volume-up", "50", "250", "soundboard"},
	{"RSS feeds", "fas fa-rss", "2", "10", "rss"},
	{"Voice roles", "fas fa-microphone", "1", "10", "voiceroles"},
	{"CC length", "fas fa-file-alt", "10k", "20k", "custom_commands"},
	{"Role-change CCs", "fas fa-user-tag", "1", "5", "custom_commands"},
	{"Slash-command CCs", "fas fa-slash", "10", "50", "custom_commands"},
	{"Subcommands / cmd", "fas fa-sitemap", "10", "25", "custom_commands"},
	{"Context menu CCs", "fas fa-mouse-pointer", "5", "15", "custom_commands"},
	{"CCs per trigger", "fas fa-layer-group", "3", "5", "custom_commands"},
	{"Reputation roles", "fas fa-medal", "5", "25", "reputation"},
	{"Automod rules", "fas fa-shield-alt", "25", "150", "automod_v2"},
	{"Template ops", "fas fa-bolt", "1M", "2.5M", "custom_commands"},
	{"DB rows / member", "fas fa-database", "50", "500", "custom_commands"},
}

// AvailablePerks leaves out perks of plugins that are not running on this instance.
func AvailablePerks() []*Perk {
	var available []*Perk
	for _, perk := range perks {
		if common.PluginLoaded(perk.pluginSysName) {
			available = append(available, perk)
		}
	}

	return available
}

func AvailableLimits() []*Limit {
	var available []*Limit
	for _, limit := range limits {
		if limit.pluginSysName == "" || common.PluginLoaded(limit.pluginSysName) {
			available = append(available, limit)
		}
	}

	return available
}
