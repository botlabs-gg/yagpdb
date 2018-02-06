package bot

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardmanager"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	// When the bot was started
	Started      = time.Now()
	Running      bool
	State        *dstate.State
	ShardManager *dshardmanager.Manager

	StateHandlerPtr *eventsystem.Handler
)

func Setup() {
	// Things may rely on state being available at this point for initialization
	State = dstate.NewState()
	eventsystem.AddHandler(HandleReady, eventsystem.EventReady)
	StateHandlerPtr = eventsystem.AddHandler(StateHandler, eventsystem.EventAll)
	eventsystem.AddHandler(HandlePresenceUpdate, eventsystem.EventPresenceUpdate)
	eventsystem.AddHandler(ConcurrentEventHandler(EventLogger.handleEvent), eventsystem.EventAll)

	eventsystem.AddHandler(RedisWrapper(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(RedisWrapper(HandleGuildDelete), eventsystem.EventGuildDelete)

	eventsystem.AddHandler(RedisWrapper(HandleGuildUpdate), eventsystem.EventGuildUpdate)
	eventsystem.AddHandler(RedisWrapper(HandleGuildRoleCreate), eventsystem.EventGuildRoleCreate)
	eventsystem.AddHandler(RedisWrapper(HandleGuildRoleUpdate), eventsystem.EventGuildRoleUpdate)
	eventsystem.AddHandler(RedisWrapper(HandleGuildRoleRemove), eventsystem.EventGuildRoleDelete)
	eventsystem.AddHandler(RedisWrapper(HandleChannelCreate), eventsystem.EventChannelCreate)
	eventsystem.AddHandler(RedisWrapper(HandleChannelUpdate), eventsystem.EventChannelUpdate)
	eventsystem.AddHandler(RedisWrapper(HandleChannelDelete), eventsystem.EventChannelDelete)
	eventsystem.AddHandler(RedisWrapper(HandleGuildMemberUpdate), eventsystem.EventGuildMemberUpdate)

	log.Info("Initializing bot plugins")
	for _, plugin := range common.Plugins {
		if botPlugin, ok := plugin.(Plugin); ok {
			botPlugin.InitBot()
			log.Info("Initialized bot plugin ", plugin.Name())
		}
	}

	log.Printf("Registered %d event handlers", eventsystem.NumHandlers(eventsystem.EventAll))
}

func Run() {

	log.Println("Running bot")

	// Set up shard manager
	ShardManager = dshardmanager.New(common.Conf.BotToken)
	ShardManager.LogChannel = os.Getenv("YAGPDB_CONNEVT_CHANNEL")
	ShardManager.StatusMessageChannel = os.Getenv("YAGPDB_CONNSTATUS_CHANNEL")
	ShardManager.Name = "YAGPDB"
	ShardManager.GuildCountsFunc = GuildCountsFunc
	ShardManager.SessionFunc = func(token string) (session *discordgo.Session, err error) {
		session, err = discordgo.New(token)
		if err != nil {
			return
		}

		session.StateEnabled = false
		session.LogLevel = discordgo.LogInformational

		return
	}

	// Only handler
	ShardManager.AddHandler(eventsystem.HandleEvent)

	State.MaxChannelMessages = 1000
	State.MaxMessageAge = time.Hour
	// State.Debug = true
	Running = true
	go ShardManager.Start()

	go mergedMessageSender()
	go MemberFetcher.Run()

	for _, p := range common.Plugins {
		starter, ok := p.(BotStarterHandler)
		if ok {
			starter.StartBot()
			log.Debug("Ran StartBot for ", p.Name())
		}
	}

	go checkConnectedGuilds()
}

func Stop(wg *sync.WaitGroup) {

	for _, v := range common.Plugins {
		stopper, ok := v.(BotStopperHandler)
		if !ok {
			continue
		}
		wg.Add(1)
		log.Debug("Sending stop event to stopper: ", v.Name())
		go stopper.StopBot(wg)
	}

	ShardManager.StopAll()
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
		if len(g) < 100 {
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

func GuildCountsFunc() []int {
	numShards := ShardManager.GetNumShards()
	result := make([]int, numShards)
	State.RLock()
	for _, v := range State.Guilds {
		parsed, _ := strconv.ParseInt(v.ID(), 10, 64)
		shard := (parsed >> 22) % int64(numShards)
		result[shard]++
	}
	State.RUnlock()

	return result
}
