package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate/inmemorytracker"
)

var Token string

func init() {
	flag.StringVar(&Token, "t", "", "The discord bot token")
}

func main() {
	flag.Parse()
	if Token == "" {
		panic("no discord bot token provided, usage: -t token-here")
	}

	if !strings.HasPrefix(Token, "Bot ") {
		Token = "Bot " + Token
	}

	session, err := discordgo.New(Token)
	if err != nil {
		panic(err)
	}
	session.Intents = []discordgo.GatewayIntent{
		discordgo.GatewayIntentGuilds,
		discordgo.GatewayIntentGuildMembers,
		discordgo.GatewayIntentGuildModeration,
		discordgo.GatewayIntentGuildExpressions,
		discordgo.GatewayIntentGuildIntegrations,
		discordgo.GatewayIntentGuildWebhooks,
		discordgo.GatewayIntentGuildInvites,
		discordgo.GatewayIntentGuildVoiceStates,
		discordgo.GatewayIntentGuildPresences,
		discordgo.GatewayIntentGuildMessages,
		discordgo.GatewayIntentGuildMessageReactions,
		discordgo.GatewayIntentGuildMessageTyping,
		discordgo.GatewayIntentDirectMessages,
		discordgo.GatewayIntentDirectMessageReactions,
		discordgo.GatewayIntentDirectMessageTyping,
	}

	self, err := session.UserMe()
	if err != nil {
		panic(err)
	}

	tracker := inmemorytracker.NewInMemoryTracker(inmemorytracker.TrackerConfig{
		BotMemberID: self.ID,
	}, 1)

	session.AddHandler(tracker.HandleEvent)

	err = session.Open()
	if err != nil {
		panic(err)
	}

	fmt.Println("Running, press ctrl-c to stop")
	select {}
}
