package bot

//go:generate sqlboiler --no-hooks psql

import (
	"runtime"
	"sync"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardmanager"
	"github.com/jonas747/dshardorchestrator/node"
	"github.com/jonas747/dstate"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot/deletequeue"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/jonas747/yagpdb/common/pubsub"
)

var (
	// When the bot was started
	Started      = time.Now()
	Enabled      bool // wether the bot is set to run at some point in this process
	Running      bool // wether the bot is currently running
	State        *dstate.State
	ShardManager *dshardmanager.Manager

	NodeConn          *node.Conn
	UsingOrchestrator bool

	MessageDeleteQueue = deletequeue.NewQueue()

	FlagNodeID string
)

var (
	confConnEventChannel         = config.RegisterOption("yagpdb.connevt.channel", "Gateway connection logging channel", 0)
	confConnStatus               = config.RegisterOption("yagpdb.connstatus.channel", "Gateway connection status channel", 0)
	confShardOrchestratorAddress = config.RegisterOption("yagpdb.orchestrator.address", "Sharding orchestrator address to connect to, if set it will be put into orchstration mode", "")
)

var (
	// the variables below specify shard clustering information, for splitting shards across multiple processes and machines
	// this is very work in process and does not work at all atm
	//
	// plugins that needs to perform shard specific tasks, not directly related to incoming gateway events should check here
	// to make sure the action they're doing is relevant to the current cluster (example: scheduled events should only run events for guilds on this cluster)

	// the total amount of shards this bot is set to use across all processes
	totalShardCount int

	// The shards running on this process, protected by the processShardsLock muted
	processShards     []int
	processShardsLock sync.RWMutex
)

func setup() {
	common.InitSchema(DBSchema, "core_bot")

	discordgo.IdentifyRatelimiter = &identifyRatelimiter{}

	// Things may rely on state being available at this point for initialization
	State = dstate.NewState()
	State.MaxChannelMessages = 1000
	State.MaxMessageAge = time.Hour
	// State.Debug = true
	State.ThrowAwayDMMessages = true
	State.TrackPrivateChannels = false
	State.CacheExpirey = time.Minute * 10
	go State.RunGCWorker()

	eventsystem.AddHandlerFirst(HandleReady, eventsystem.EventReady)
	eventsystem.AddHandlerSecond(StateHandler, eventsystem.EventAll)

	eventsystem.AddHandlerAsyncLast(EventLogger.handleEvent, eventsystem.EventAll)

	eventsystem.AddHandlerAsyncLast(HandleGuildCreate, eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLast(HandleGuildDelete, eventsystem.EventGuildDelete)

	eventsystem.AddHandlerAsyncLast(HandleGuildUpdate, eventsystem.EventGuildUpdate)
	eventsystem.AddHandlerAsyncLast(HandleGuildRoleCreate, eventsystem.EventGuildRoleCreate)
	eventsystem.AddHandlerAsyncLast(HandleGuildRoleUpdate, eventsystem.EventGuildRoleUpdate)
	eventsystem.AddHandlerAsyncLast(HandleGuildRoleRemove, eventsystem.EventGuildRoleDelete)
	eventsystem.AddHandlerAsyncLast(HandleChannelCreate, eventsystem.EventChannelCreate)
	eventsystem.AddHandlerAsyncLast(HandleChannelUpdate, eventsystem.EventChannelUpdate)
	eventsystem.AddHandlerAsyncLast(HandleChannelDelete, eventsystem.EventChannelDelete)
	eventsystem.AddHandlerAsyncLast(HandleGuildMemberUpdate, eventsystem.EventGuildMemberUpdate)
	eventsystem.AddHandlerAsyncLast(HandleGuildMemberAdd, eventsystem.EventGuildMemberAdd)
	eventsystem.AddHandlerAsyncLast(HandleGuildMemberRemove, eventsystem.EventGuildMemberRemove)
	eventsystem.AddHandlerAsyncLast(HandleGuildMembersChunk, eventsystem.EventGuildMembersChunk)
	eventsystem.AddHandlerAsyncLast(HandleReactionAdd, eventsystem.EventMessageReactionAdd)
	eventsystem.AddHandlerAsyncLast(HandleMessageCreate, eventsystem.EventMessageCreate)
	eventsystem.AddHandlerAsyncLast(HandleResumed, eventsystem.EventResumed)
}

func Run() {
	setup()

	logger.Println("Running bot")

	connEvtChannel := confConnEventChannel.GetInt()
	connStatusChannel := confConnStatus.GetInt()

	// Set up shard manager
	ShardManager = dshardmanager.New(common.GetBotToken())
	ShardManager.LogChannel = int64(connEvtChannel)
	ShardManager.StatusMessageChannel = int64(connStatusChannel)
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

	orcheStratorAddress := confShardOrchestratorAddress.GetString()
	if orcheStratorAddress != "" {
		UsingOrchestrator = true
		logger.Infof("Set to use orchestrator at address: %s", orcheStratorAddress)
	} else {
		logger.Info("Running standalone without any orchestrator")
		SetupStandalone()
	}

	go MemberFetcher.Run()
	go mergedMessageSender()

	Running = true

	if UsingOrchestrator {
		// TODO
		NodeConn = node.NewNodeConn(&NodeImpl{}, orcheStratorAddress, common.VERSION, FlagNodeID, nil)
		NodeConn.Run()
	} else {
		go ShardManager.Start()
		InitPlugins()
	}

	// if masterAddr != "" {
	// 	stateLock.Lock()
	// 	state = StateWaitingHelloMaster
	// 	stateLock.Unlock()

	// 	logger.Println("Connecting to master at ", masterAddr, ", wont start until connected and told to start")
	// 	var err error
	// 	SlaveClient, err = slave.ConnectToMaster(&SlaveImpl{}, masterAddr)
	// 	if err != nil {
	// 		logger.WithError(err).Error("Failed connecting to master")
	// 		os.Exit(1)
	// 	}
	// } else {
	// 	stateLock.Lock()
	// 	state = StateRunningNoMaster
	// 	stateLock.Unlock()

	// 	InitPlugins()

	// 	logger.Println("Running normally without a master")
	// 	go ShardManager.Start()
	// 	go MonitorLoading()
	// }

	// for _, p := range common.Plugins {
	// 	starter, ok := p.(BotStarterHandler)
	// 	if ok {
	// 		starter.StartBot()
	// 		logger.Debug("Ran StartBot for ", p.Name())
	// 	}
	// }
}

func SetupStandalone() {
	shardCount, err := ShardManager.GetRecommendedCount()
	if err != nil {
		panic("Failed getting shard count: " + err.Error())
	}
	totalShardCount = shardCount

	EventLogger.init(shardCount)
	go EventLogger.run()

	processShards = make([]int, totalShardCount)
	for i := 0; i < totalShardCount; i++ {
		processShards[i] = i
	}

	err = common.RedisPool.Do(retryableredis.FlatCmd(nil, "SET", "yagpdb_total_shards", shardCount))
	if err != nil {
		logger.WithError(err).Error("failed setting shard count")
	}
}

func InitPlugins() {
	pubsub.AddHandler("bot_status_changed", func(evt *pubsub.Event) {
		updateAllShardStatuses()
	}, nil)

	// Initialize all plugins
	for _, plugin := range common.Plugins {
		if initBot, ok := plugin.(BotInitHandler); ok {
			initBot.BotInit()
		}
	}

	// Initialize all plugins late
	for _, plugin := range common.Plugins {
		if initBot, ok := plugin.(LateBotInitHandler); ok {
			initBot.LateBotInit()
		}
	}

	if common.Statsd != nil {
		go goroutineLogger()
	}

	go loopCheckAdmins()
	watchMemusage()
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
			logger.Debug("Calling bot stopper for: ", v.PluginInfo().Name)
			go stopper.StopBot(wg)
		}

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

// Standard implementation of the GatewayIdentifyRatelimiter
type identifyRatelimiter struct {
	ch   chan bool
	once sync.Once
}

func (rl *identifyRatelimiter) RatelimitIdentify() {
	const key = "yagpdb.gateway.identify.limit"
	for {
		var resp string
		err := common.RedisPool.Do(retryableredis.Cmd(&resp, "SET", key, "1", "EX", "5", "NX"))
		if err != nil {
			logger.WithError(err).Error("failed ratelimiting gateway")
			time.Sleep(time.Second)
			continue
		}

		if resp == "OK" {
			return // success
		}

		// otherwise a identify was sent by someone else last 5 seconds
		time.Sleep(time.Second)
	}
}

func goroutineLogger() {
	t := time.NewTicker(time.Second * 10)
	for {
		<-t.C

		num := runtime.NumGoroutine()
		common.Statsd.Gauge("yagpdb.numgoroutine", float64(num), nil, 1)
	}
}
