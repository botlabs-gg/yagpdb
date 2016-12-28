package bot

//go:generate go run ../cmd/gen/bot_wrappers.go -o wrappers.go

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/common"
	"sync"
	"time"
)

var (
	// When the bot was started
	Started = time.Now()
	Running bool
	State   *dstate.State
)

func Setup() {
	// Things may rely on state being available at this point for initialization
	State = dstate.NewState()

	log.Info("Initializing bot plugins")
	for _, plugin := range Plugins {
		plugin.InitBot()
		log.Info("Initialized bot plugin ", plugin.Name())
	}
}

func Run() {

	log.Println("Running bot")
	common.BotSession.AddHandler(HandleReady)
	common.BotSession.AddHandler(CustomGuildCreate(HandleGuildCreate))
	common.BotSession.AddHandler(CustomGuildDelete(HandleGuildDelete))

	common.BotSession.AddHandler(CustomGuildUpdate(HandleGuildUpdate))
	common.BotSession.AddHandler(CustomGuildRoleCreate(HandleGuildRoleCreate))
	common.BotSession.AddHandler(CustomGuildRoleUpdate(HandleGuildRoleUpdate))
	common.BotSession.AddHandler(CustomGuildRoleDelete(HandleGuildRoleRemove))
	common.BotSession.AddHandler(CustomChannelCreate(HandleChannelCreate))
	common.BotSession.AddHandler(CustomChannelUpdate(HandleChannelUpdate))
	common.BotSession.AddHandler(CustomChannelDelete(HandleChannelDelete))

	// common.BotSession.LogLevel = discordgo.LogDebug
	// common.BotSession.Debug = true

	State.MaxChannelMessages = 1000
	State.MaxMessageAge = time.Hour
	// State.Debug = true

	common.BotSession.StateEnabled = false
	common.BotSession.AddHandler(State.HandleEvent)

	common.BotSession.LogLevel = discordgo.LogInformational
	err := common.BotSession.Open()
	if err != nil {
		panic(err)
	}

	Running = true

	go mergedMessageSender()

	for _, p := range Plugins {
		starter, ok := p.(BotStarterHandler)
		if ok {
			starter.StartBot()
			log.Info("Ran StartBot for ", p.Name())
		}
	}

	go checkConnectedGuilds()
}

func Stop(wg *sync.WaitGroup) {

	for _, v := range Plugins {
		stopper, ok := v.(BotStopperHandler)
		if !ok {
			continue
		}
		wg.Add(1)
		log.Info("Sending stop event to stopper: ", v.Name())
		go stopper.StopBot(wg)
	}

	common.BotSession.Close()
	wg.Done()
}

// checks all connected guilds and emites guildremoved on those no longer connected
func checkConnectedGuilds() {
	log.Info("Checking joined guilds")

	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving connection from redis pool")
		return
	}

	currentlyConnected, err := client.Cmd("SMEMBERS", "connected_guilds").List()
	if err != nil {
		log.WithError(err).Error("Failed retrieving currently connected guilds")
		return
	}

	guilds := make([]*discordgo.UserGuild, 0)
	after := ""

	for {
		g, err := common.BotSession.UserGuilds(100, "", after)
		if err != nil {
			log.WithError(err).Error("Userguilds failed")
			return
		}

		guilds = append(guilds, g...)
		if len(guilds) < 100 {
			break
		}

		after = g[len(g)-1].ID
	}

OUTER:
	for _, gID := range currentlyConnected {
		for _, g := range guilds {
			if g.ID == gID {
				continue OUTER
			}
		}

		err := client.Cmd("SREM", "connected_guilds", gID).Err
		if err != nil {
			log.WithError(err).Error("Failed removing guild from connected guilds")
		} else {
			EmitGuildRemoved(client, gID)
			log.WithField("guild", gID).Info("Removed from guild when offline")
		}
	}
}
