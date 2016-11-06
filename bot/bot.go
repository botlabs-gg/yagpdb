package bot

//go:generate go run ../cmd/gen/bot_wrappers.go -o wrappers.go

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"time"
)

var (
	// When the bot was started
	Started = time.Now()
	Running bool
)

func Setup() {
	common.BotSession.State.MaxMessageCount = 1000

	log.Info("Initializing bot plugins")
	for _, plugin := range plugins {
		plugin.InitBot()
		log.WithField("plugin", plugin.Name()).Info("Initialized plugin")
	}
}

func Run() {
	log.Println("Running bot")
	common.BotSession.AddHandler(HandleReady)
	common.BotSession.AddHandler(CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(CustomGuildDelete(HandleGuildDelete))
	// common.BotSession.AddHandler(CustomGuildMembersChunk(HandleGuildMembersChunk))

	// common.BotSession.Debug = true
	// common.BotSession.LogLevel = discordgo.LogDebug

	err := common.BotSession.Open()
	if err != nil {
		panic(err)
	}

	Running = true

	go mergedMessageSender()
	// go guildMembersRequester()
	go pollEvents()
}
