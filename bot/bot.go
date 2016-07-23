package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/common"
	"log"
)

var (
	Config    *common.Config
	Session   *discordgo.Session
	RedisPool *pool.Pool
)

func Run() {

	Config = common.Conf
	Session = common.BotSession
	RedisPool = common.RedisPool

	log.Println("Running bot...")
	for _, plugin := range plugins {
		plugin.InitBot()
		log.Println("Initialized bot plugin", plugin.Name())
	}

	Session.AddHandler(HandleReady)
	Session.AddHandler(HandleGuildCreate)
	Session.AddHandler(HandleGuildDelete)

	// Session.Debug = true
	// Session.LogLevel = discordgo.LogDebug

	err := Session.Open()
	if err != nil {
		log.Println("Failed opening bot connection", err)
	}
}
