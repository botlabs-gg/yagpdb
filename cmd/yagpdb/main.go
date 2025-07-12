package main

import (
	"github.com/RhykerWells/yagpdb/v2/analytics"
	"github.com/RhykerWells/yagpdb/v2/antiphishing"
	"github.com/RhykerWells/yagpdb/v2/common/featureflags"
	"github.com/RhykerWells/yagpdb/v2/common/prom"
	"github.com/RhykerWells/yagpdb/v2/common/run"
	"github.com/RhykerWells/yagpdb/v2/lib/confusables"
	"github.com/RhykerWells/yagpdb/v2/trivia"
	"github.com/RhykerWells/yagpdb/v2/web/discorddata"

	// Core yagpdb packages

	"github.com/RhykerWells/yagpdb/v2/admin"
	"github.com/RhykerWells/yagpdb/v2/bot/paginatedmessages"
	"github.com/RhykerWells/yagpdb/v2/common/internalapi"
	"github.com/RhykerWells/yagpdb/v2/common/scheduledevents2"

	// Plugin imports
	"github.com/RhykerWells/yagpdb/v2/automod"
	"github.com/RhykerWells/yagpdb/v2/automod_legacy"
	"github.com/RhykerWells/yagpdb/v2/autorole"
	"github.com/RhykerWells/yagpdb/v2/commands"
	"github.com/RhykerWells/yagpdb/v2/customcommands"
	"github.com/RhykerWells/yagpdb/v2/discordlogger"
	"github.com/RhykerWells/yagpdb/v2/roblox"
	"github.com/RhykerWells/yagpdb/v2/logs"
	"github.com/RhykerWells/yagpdb/v2/moderation"
	"github.com/RhykerWells/yagpdb/v2/notifications"
	"github.com/RhykerWells/yagpdb/v2/premium"
	"github.com/RhykerWells/yagpdb/v2/premium/discordpremiumsource"
	"github.com/RhykerWells/yagpdb/v2/premium/patreonpremiumsource"
	"github.com/RhykerWells/yagpdb/v2/reminders"
	"github.com/RhykerWells/yagpdb/v2/rolecommands"
	"github.com/RhykerWells/yagpdb/v2/safebrowsing"
	"github.com/RhykerWells/yagpdb/v2/serverstats"
	"github.com/RhykerWells/yagpdb/v2/stdcommands"
	"github.com/RhykerWells/yagpdb/v2/tickets"
	"github.com/RhykerWells/yagpdb/v2/timezonecompanion"
	"github.com/RhykerWells/yagpdb/v2/verification"
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
	moderation.RegisterPlugin()
	automod_legacy.RegisterPlugin()
	automod.RegisterPlugin()
	logs.RegisterPlugin()
	autorole.RegisterPlugin()
	reminders.RegisterPlugin()
	rolecommands.RegisterPlugin()
	tickets.RegisterPlugin()
	verification.RegisterPlugin()
	premium.RegisterPlugin()
	patreonpremiumsource.RegisterPlugin()
	discordpremiumsource.RegisterPlugin()
	scheduledevents2.RegisterPlugin()
	timezonecompanion.RegisterPlugin()
	admin.RegisterPlugin()
	internalapi.RegisterPlugin()
	prom.RegisterPlugin()
	featureflags.RegisterPlugin()
	trivia.RegisterPlugin()
	roblox.RegisterPlugin()
	// Register confusables replacer
	confusables.Init()

	run.Run()
}
