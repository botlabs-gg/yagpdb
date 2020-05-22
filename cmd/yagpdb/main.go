package main

import (
	"github.com/jonas747/yagpdb/analytics"
	"github.com/jonas747/yagpdb/common/featureflags"
	"github.com/jonas747/yagpdb/common/prom"
	"github.com/jonas747/yagpdb/common/run"

	// Core yagpdb packages

	"github.com/jonas747/yagpdb/admin"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common/internalapi"
	"github.com/jonas747/yagpdb/common/scheduledevents2"

	// Plugin imports
	"github.com/jonas747/yagpdb/automod"
	"github.com/jonas747/yagpdb/automod_legacy"
	"github.com/jonas747/yagpdb/autorole"
	"github.com/jonas747/yagpdb/aylien"
	"github.com/jonas747/yagpdb/cah"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/customcommands"
	"github.com/jonas747/yagpdb/discordlogger"
	"github.com/jonas747/yagpdb/logs"
	"github.com/jonas747/yagpdb/moderation"
	"github.com/jonas747/yagpdb/notifications"
	"github.com/jonas747/yagpdb/premium"
	"github.com/jonas747/yagpdb/premium/patreonpremiumsource"
	"github.com/jonas747/yagpdb/reddit"
	"github.com/jonas747/yagpdb/reminders"
	"github.com/jonas747/yagpdb/reputation"
	"github.com/jonas747/yagpdb/rolecommands"
	"github.com/jonas747/yagpdb/rsvp"
	"github.com/jonas747/yagpdb/safebrowsing"
	"github.com/jonas747/yagpdb/serverstats"
	"github.com/jonas747/yagpdb/soundboard"
	"github.com/jonas747/yagpdb/stdcommands"
	"github.com/jonas747/yagpdb/streaming"
	"github.com/jonas747/yagpdb/tickets"
	"github.com/jonas747/yagpdb/timezonecompanion"
	"github.com/jonas747/yagpdb/twitter"
	"github.com/jonas747/yagpdb/verification"
	"github.com/jonas747/yagpdb/youtube"
	// External plugins
)

func main() {

	run.Init()

	//BotSession.LogLevel = discordgo.LogInformational
	paginatedmessages.RegisterPlugin()

	// Setup plugins
	analytics.RegisterPlugin()
	safebrowsing.RegisterPlugin()
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
