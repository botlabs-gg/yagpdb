package bot

import (
	"flag"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardmanager"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot/deletequeue"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/jonas747/yagpdb/master/slave"
	log "github.com/sirupsen/logrus"
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

	SlaveClient *slave.Conn

	state              int
	stateLock          sync.Mutex
	MessageDeleteQueue = deletequeue.NewQueue()
)

var (
	// the variables below specify shard clustering information, for splitting shards across multiple processes and machines
	// this is very work in process and does not work at all atm
	//
	// plugins that needs to perform shard specific tasks, not directly related to incoming gateway events should check here
	// to make sure the action they're doing is relevant to the current cluster (example: scheduled events should only run events for guilds on this cluster)

	// the total amount of shards this bot is set to use across all processes
	TotalShardCount int

	// the number of shards running on this process
	ProcessShardCount int

	// the offset of the running shards
	// example: if we were running shard 4 and 5, the offset would be 4 and the running shard count 2
	RunShardOffset int
)

func init() {
	flag.IntVar(&TotalShardCount, "totalshards", 0, "TODO: Total shards the bot has, <1 for auto")
	flag.IntVar(&ProcessShardCount, "runshards", 0, "TODO: The number of shards this process should run, <1 for all")
	flag.IntVar(&RunShardOffset, "runshardsoffset", 0, "TODO: The start offset, e.g if we wanna run shard 10-19 then runshards would be 10 and runshardsoffset would be 10")
}

const (
	StateRunningNoMaster   int = iota
	StateRunningWithMaster     // Fully started

	StateWaitingHelloMaster
	StateSoftStarting
	StateShardMigrationTo
	StateShardMigrationFrom
)

func setup() {
	// Things may rely on state being available at this point for initialization
	State = dstate.NewState()
	State.MaxChannelMessages = 1000
	State.MaxMessageAge = time.Hour
	// State.Debug = true
	State.ThrowAwayDMMessages = true
	State.CacheExpirey = time.Minute * 10
	go State.RunGCWorker()

	eventsystem.AddHandler(HandleReady, eventsystem.EventReady)
	StateHandlerPtr = eventsystem.AddHandler(StateHandler, eventsystem.EventAll)
	eventsystem.ConcurrentAfter = StateHandlerPtr

	eventsystem.AddHandler(ConcurrentEventHandler(EventLogger.handleEvent), eventsystem.EventAll)

	eventsystem.AddHandler(HandleGuildCreate, eventsystem.EventGuildCreate)
	eventsystem.AddHandler(HandleGuildDelete, eventsystem.EventGuildDelete)

	eventsystem.AddHandler(HandleGuildUpdate, eventsystem.EventGuildUpdate)
	eventsystem.AddHandler(HandleGuildRoleCreate, eventsystem.EventGuildRoleCreate)
	eventsystem.AddHandler(HandleGuildRoleUpdate, eventsystem.EventGuildRoleUpdate)
	eventsystem.AddHandler(HandleGuildRoleRemove, eventsystem.EventGuildRoleDelete)
	eventsystem.AddHandler(HandleChannelCreate, eventsystem.EventChannelCreate)
	eventsystem.AddHandler(HandleChannelUpdate, eventsystem.EventChannelUpdate)
	eventsystem.AddHandler(HandleChannelDelete, eventsystem.EventChannelDelete)
	eventsystem.AddHandler(HandleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	eventsystem.AddHandler(HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandler(HandleGuildMembersChunk, eventsystem.EventGuildMembersChunk)
}

func Run() {
	setup()

	log.Println("Running bot")

	connEvtChannel, _ := strconv.ParseInt(os.Getenv("YAGPDB_CONNEVT_CHANNEL"), 10, 64)
	connStatusChannel, _ := strconv.ParseInt(os.Getenv("YAGPDB_CONNSTATUS_CHANNEL"), 10, 64)

	// Set up shard manager
	ShardManager = dshardmanager.New(common.Conf.BotToken)
	ShardManager.LogChannel = connEvtChannel
	ShardManager.StatusMessageChannel = connStatusChannel
	ShardManager.Name = "YAGPDB"
	ShardManager.GuildCountsFunc = GuildCountsFunc
	ShardManager.SessionFunc = func(token string) (session *discordgo.Session, err error) {
		session, err = discordgo.New(token)
		if err != nil {
			return
		}

		session.StateEnabled = false
		session.LogLevel = discordgo.LogInformational
		session.SyncEvents = true

		// Certain discordgo internals expect this to be present
		// but in case of shard migration it's not, so manually assign it here
		session.State.Ready = discordgo.Ready{
			User: &discordgo.SelfUser{
				User: common.BotUser,
			},
		}

		return
	}

	// Only handler
	ShardManager.AddHandler(eventsystem.HandleEvent)

	if TotalShardCount < 1 || ProcessShardCount < 1 {
		// not enough shard clustering info provided, run all shards we are reccomended from discord
		shardCount, err := ShardManager.GetRecommendedCount()
		if err != nil {
			panic("Failed getting shard count: " + err.Error())
		}
		TotalShardCount = shardCount
		ProcessShardCount = shardCount
		RunShardOffset = 0
	} else {
		// TODO
	}

	log.Infof("Bot clustering info (WIP): Tot. Shards: %d, Running shards: %d, Shard offset: %d", TotalShardCount, ProcessShardCount, RunShardOffset)

	EventLogger.init(ProcessShardCount)
	go EventLogger.run()

	for i := RunShardOffset; i < ProcessShardCount; i++ {
		waitingReadies = append(waitingReadies, i)
	}

	Running = true

	// go ShardManager.Start()
	go MemberFetcher.Run()
	go mergedMessageSender()

	masterAddr := os.Getenv("YAGPDB_MASTER_CONNECT_ADDR")
	if masterAddr != "" {
		stateLock.Lock()
		state = StateWaitingHelloMaster
		stateLock.Unlock()

		log.Println("Connecting to master at ", masterAddr, ", wont start until connected and told to start")
		var err error
		SlaveClient, err = slave.ConnectToMaster(&SlaveImpl{}, masterAddr)
		if err != nil {
			log.WithError(err).Error("Failed connecting to master")
			os.Exit(1)
		}
	} else {
		stateLock.Lock()
		state = StateRunningNoMaster
		stateLock.Unlock()

		InitPlugins()

		log.Println("Running normally without a master")
		go ShardManager.Start()
		go MonitorLoading()
	}

	// for _, p := range common.Plugins {
	// 	starter, ok := p.(BotStarterHandler)
	// 	if ok {
	// 		starter.StartBot()
	// 		log.Debug("Ran StartBot for ", p.Name())
	// 	}
	// }
}

func MonitorLoading() {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		<-t.C

		waitingGuildsMU.Lock()
		numWaitingGuilds := len(waitingGuilds)
		numWaitingShards := len(waitingReadies)
		waitingGuildsMU.Unlock()

		log.Infof("Starting up... GC's Remaining: %d, Shards remaining: %d", numWaitingGuilds, numWaitingShards)

		if numWaitingShards == 0 {
			return
		}
	}
}

func InitPlugins() {
	// Initialize all plugins
	for _, plugin := range common.Plugins {
		if initBot, ok := plugin.(BotInitHandler); ok {
			initBot.BotInit()
		}
	}
}

func BotStarted() {
	for _, p := range common.Plugins {
		starter, ok := p.(BotStartedHandler)
		if ok {
			starter.BotStarted()
			log.Debug("Ran BotStarted for ", p.Name())
		}
	}

	go scheduledevents.Run()
	go loopCheckAdmins()
}

var stopOnce sync.Once

func StopAllPlugins(wg *sync.WaitGroup) {
	stopOnce.Do(func() {
		for _, v := range common.Plugins {
			stopper, ok := v.(BotStopperHandler)
			if !ok {
				continue
			}
			wg.Add(1)
			log.Debug("Calling bot stopper for: ", v.Name())
			go stopper.StopBot(wg)
		}

		wg.Add(1)
		go scheduledevents.Stop(wg)
		close(stopRunCheckAdmins)
	})
}

func Stop(wg *sync.WaitGroup) {
	StopAllPlugins(wg)
	ShardManager.StopAll()
	wg.Done()
}

func GuildCountsFunc() []int {
	numShards := ShardManager.GetNumShards()
	result := make([]int, numShards)
	State.RLock()
	for _, v := range State.Guilds {
		shard := (v.ID >> 22) % int64(numShards)
		result[shard]++
	}
	State.RUnlock()

	return result
}

// IsGuildOnCurrentProcess returns whether the guild is on one of the shards for this process
func IsGuildOnCurrentProcess(guildID int64) bool {
	shardID := int((guildID >> 22) % int64(TotalShardCount))
	return shardID >= RunShardOffset && shardID < RunShardOffset+ProcessShardCount
}
