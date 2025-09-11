package main

import (
	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/antiphishing"
	"github.com/botlabs-gg/yagpdb/v2/bulkrole"
	"github.com/botlabs-gg/yagpdb/v2/common/featureflags"
	"github.com/botlabs-gg/yagpdb/v2/common/prom"
	"github.com/botlabs-gg/yagpdb/v2/common/run"
	"github.com/botlabs-gg/yagpdb/v2/lib/confusables"
	"github.com/botlabs-gg/yagpdb/v2/trivia"
	"github.com/botlabs-gg/yagpdb/v2/web/discorddata"

	// Core yagpdb packages

	"github.com/botlabs-gg/yagpdb/v2/admin"
	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/common/internalapi"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"

	// Plugin imports
	"github.com/botlabs-gg/yagpdb/v2/automod"
	"github.com/botlabs-gg/yagpdb/v2/automod_legacy"
	"github.com/botlabs-gg/yagpdb/v2/autorole"
	"github.com/botlabs-gg/yagpdb/v2/cah"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/customcommands"
	"github.com/botlabs-gg/yagpdb/v2/discordlogger"
	"github.com/botlabs-gg/yagpdb/v2/logs"
	"github.com/botlabs-gg/yagpdb/v2/moderation"
	"github.com/botlabs-gg/yagpdb/v2/notifications"
	"github.com/botlabs-gg/yagpdb/v2/personalizer"
	"github.com/botlabs-gg/yagpdb/v2/premium"
	"github.com/botlabs-gg/yagpdb/v2/premium/discordpremiumsource"
	"github.com/botlabs-gg/yagpdb/v2/premium/patreonpremiumsource"
	"github.com/botlabs-gg/yagpdb/v2/reddit"
	"github.com/botlabs-gg/yagpdb/v2/reminders"
	"github.com/botlabs-gg/yagpdb/v2/reputation"
	"github.com/botlabs-gg/yagpdb/v2/rolecommands"
	"github.com/botlabs-gg/yagpdb/v2/rss"
	"github.com/botlabs-gg/yagpdb/v2/rsvp"
	"github.com/botlabs-gg/yagpdb/v2/safebrowsing"
	"github.com/botlabs-gg/yagpdb/v2/serverstats"
	"github.com/botlabs-gg/yagpdb/v2/soundboard"
	"github.com/botlabs-gg/yagpdb/v2/stdcommands"
	"github.com/botlabs-gg/yagpdb/v2/streaming"
	"github.com/botlabs-gg/yagpdb/v2/tickets"
	"github.com/botlabs-gg/yagpdb/v2/timezonecompanion"
	"github.com/botlabs-gg/yagpdb/v2/twitter"
	"github.com/botlabs-gg/yagpdb/v2/verification"
	"github.com/botlabs-gg/yagpdb/v2/youtube"
	// External plugins
)

func main() {

	run.Init()

	//BotSession.LogLevel = discordgo.LogInformational
	paginatedmessages.RegisterPlugin()
	discorddata.RegisterPlugin()

	// Setup plugins
	analytics.RegisterPlugin()
	safebrowsing.RegisterPlugin()
	antiphishing.RegisterPlugin()
	discordlogger.Register()
	commands.RegisterPlugin()
	stdcommands.RegisterPlugin()
	serverstats.RegisterPlugin()
	notifications.RegisterPlugin()
	customcommands.RegisterPlugin()
	reddit.RegisterPlugin()
	moderation.RegisterPlugin()
	reputation.RegisterPlugin()
	streaming.RegisterPlugin()
	automod_legacy.RegisterPlugin()
	automod.RegisterPlugin()
	logs.RegisterPlugin()
	autorole.RegisterPlugin()
	reminders.RegisterPlugin()
	soundboard.RegisterPlugin()
	youtube.RegisterPlugin()
	rolecommands.RegisterPlugin()
	cah.RegisterPlugin()
	tickets.RegisterPlugin()
	verification.RegisterPlugin()
	premium.RegisterPlugin()
	patreonpremiumsource.RegisterPlugin()
	discordpremiumsource.RegisterPlugin()
	scheduledevents2.RegisterPlugin()
	twitter.RegisterPlugin()
	rsvp.RegisterPlugin()
	timezonecompanion.RegisterPlugin()
	admin.RegisterPlugin()
	internalapi.RegisterPlugin()
	prom.RegisterPlugin()
	featureflags.RegisterPlugin()
	trivia.RegisterPlugin()
	rss.RegisterPlugin()
	bulkrole.RegisterPlugin()
	personalizer.RegisterPlugin()

	// Register confusables replacer
	confusables.Init()

	run.Run()
}
