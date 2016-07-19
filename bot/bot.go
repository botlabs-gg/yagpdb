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
	log.Println("Running bot...")
	for _, plugin := range plugins {
		plugin.InitBot()
		log.Println("Initialised bot plugin", plugin.Name())
	}

	Session.AddHandler(HandleReady)
	Session.AddHandler(HandleGuildCreate)
	Session.AddHandler(HandleGuildDelete)

	err := Session.Open()
	if err != nil {
		log.Println("Failed opening bot connection", err)
	}
}
