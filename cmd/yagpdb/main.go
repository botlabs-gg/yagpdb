package main

import (
	"github.com/botlabs-gg/yagpdb/analytics"
	"github.com/botlabs-gg/yagpdb/antiphishing"
	"github.com/botlabs-gg/yagpdb/common/featureflags"
	"github.com/botlabs-gg/yagpdb/common/prom"
	"github.com/botlabs-gg/yagpdb/common/run"
	"github.com/botlabs-gg/yagpdb/web/discorddata"

	// Core yagpdb packages

	"github.com/botlabs-gg/yagpdb/admin"
	"github.com/botlabs-gg/yagpdb/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/common/internalapi"
	"github.com/botlabs-gg/yagpdb/common/scheduledevents2"

	// Plugin imports
	"github.com/botlabs-gg/yagpdb/automod"
	"github.com/botlabs-gg/yagpdb/automod_legacy"
	"github.com/botlabs-gg/yagpdb/autorole"
	"github.com/botlabs-gg/yagpdb/aylien"
	"github.com/botlabs-gg/yagpdb/cah"
	"github.com/botlabs-gg/yagpdb/commands"
	"github.com/botlabs-gg/yagpdb/customcommands"
	"github.com/botlabs-gg/yagpdb/discordlogger"
	"github.com/botlabs-gg/yagpdb/logs"
	"github.com/botlabs-gg/yagpdb/moderation"
	"github.com/botlabs-gg/yagpdb/notifications"
	"github.com/botlabs-gg/yagpdb/premium"
	"github.com/botlabs-gg/yagpdb/premium/patreonpremiumsource"
	"github.com/botlabs-gg/yagpdb/reddit"
	"github.com/botlabs-gg/yagpdb/reminders"
	"github.com/botlabs-gg/yagpdb/reputation"
	"github.com/botlabs-gg/yagpdb/rolecommands"
	"github.com/botlabs-gg/yagpdb/rsvp"
	"github.com/botlabs-gg/yagpdb/safebrowsing"
	"github.com/botlabs-gg/yagpdb/serverstats"
	"github.com/botlabs-gg/yagpdb/soundboard"
	"github.com/botlabs-gg/yagpdb/stdcommands"
	"github.com/botlabs-gg/yagpdb/streaming"
	"github.com/botlabs-gg/yagpdb/tickets"
	"github.com/botlabs-gg/yagpdb/timezonecompanion"
	"github.com/botlabs-gg/yagpdb/twitter"
	"github.com/botlabs-gg/yagpdb/verification"
	"github.com/botlabs-gg/yagpdb/youtube"
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
	aylien.RegisterPlugin()
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
	scheduledevents2.RegisterPlugin()
	twitter.RegisterPlugin()
	rsvp.RegisterPlugin()
	timezonecompanion.RegisterPlugin()
	admin.RegisterPlugin()
	internalapi.RegisterPlugin()
	prom.RegisterPlugin()
	featureflags.RegisterPlugin()

	run.Run()
}
