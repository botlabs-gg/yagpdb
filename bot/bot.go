package bot

//go:generate go run ../cmd/gen/bot_wrappers.go -o wrappers.go

import (
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

var (
	Config    *common.Config
	Session   *discordgo.Session
	RedisPool *pool.Pool

	// When the bot was started
	Started = time.Now()
	Running bool
)

func Setup() {
	Config = common.Conf
	Session = common.BotSession
	RedisPool = common.RedisPool

	Session.State.MaxMessageCount = 1000

	log.Println("Initializing bot plugins...")
	for _, plugin := range plugins {
		plugin.InitBot()
		log.Println("Initialized bot plugin", plugin.Name())
	}
}

func Run() {
	log.Println("Running bot")
	Session.AddHandler(HandleReady)
	Session.AddHandler(CustomGuildCreate(HandleGuildCreate))
	Session.AddHandler(CustomGuildDelete(HandleGuildDelete))

	// Session.Debug = true
	// Session.LogLevel = discordgo.LogDebug

	err := Session.Open()
	if err != nil {
		log.Println("Failed opening bot connection", err)
	}

	Running = true
}
