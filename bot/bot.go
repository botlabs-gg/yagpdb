package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/yagpdb/common"
	"log"
)

var (
	Config  *common.Config
	Session *discordgo.Session
)

func Run() {
	var err error
	Session, err = discordgo.New(Config.BotToken)
	if err != nil {
		log.Println("Error creating bot session", err)
		return
	}

	for _, plugin := range plugins {
		plugin.InitBot()
		log.Println("Initialised bot plugin", plugin.Name())
	}

	err = Session.Open()
	if err != nil {
		log.Println("Failed opening bot connection", err)
	}
}
