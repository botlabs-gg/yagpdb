package bot

//go:generate go run ../cmd/gen/bot_wrappers.go -o wrappers.go

import (
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

var (
	// When the bot was started
	Started = time.Now()
	Running bool
)

func Setup() {
	common.BotSession.State.MaxMessageCount = 1000

	log.Println("Initializing bot plugins...")
	for _, plugin := range plugins {
		plugin.InitBot()
		log.Println("Initialized bot plugin", plugin.Name())
	}
}

func Run() {
	log.Println("Running bot")
	common.BotSession.AddHandler(HandleReady)
	common.BotSession.AddHandler(CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(CustomGuildDelete(HandleGuildDelete))

	// common.BotSession.Debug = true
	// common.BotSession.LogLevel = discordgo.LogDebug

	err := common.BotSession.Open()
	if err != nil {
		log.Println("Failed opening bot connection", err)
	}

	Running = true
}
