package bot

//go:generate sqlboiler --no-hooks psql

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/bot/shardmemberfetcher"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate/inmemorytracker"
	dshardmanager "github.com/botlabs-gg/yagpdb/v2/lib/jdshardmanager"
	"github.com/mediocregopher/radix/v3"
)

var (
	// When the bot was started
	Started      = time.Now()
	Enabled      bool // wether the bot is set to run at some point in this process
	Running      bool // wether the bot is currently running
	State        dstate.StateTracker
	stateTracker *inmemorytracker.InMemoryTracker

	ShardManager *dshardmanager.Manager

	NodeConn          *node.Conn
	UsingOrchestrator bool
)

var (
	confConnEventChannel         = config.RegisterOption("yagpdb.connevt.channel", "Gateway connection logging channel", 0)
	confConnStatus               = config.RegisterOption("yagpdb.connstatus.channel", "Gateway connection status channel", 0)
	confShardOrchestratorAddress = config.RegisterOption("yagpdb.orchestrator.address", "Sharding orchestrator address to connect to, if set it will be put into orchstration mode", "")

	confFixedShardingConfig = config.RegisterOption("yagpdb.sharding.fixed_config", "Fixed sharding config, mostly used during testing, allows you to run a single shard, the format is: 'id,count', example: '0,10'", "")

	usingFixedSharding bool
	fixedShardingID    int

	// Note yags is using priviledged intents
	gatewayIntentsUsed = []discordgo.GatewayIntent{
		discordgo.GatewayIntentGuilds,
		discordgo.GatewayIntentGuildMembers,
		discordgo.GatewayIntentGuildModeration,
		discordgo.GatewayIntentGuildExpressions,
		discordgo.GatewayIntentGuildVoiceStates,
		discordgo.GatewayIntentGuildPresences,
		discordgo.GatewayIntentGuildMessages,
		discordgo.GatewayIntentGuildMessageReactions,
		discordgo.GatewayIntentDirectMessages,
		discordgo.GatewayIntentDirectMessageReactions,
		discordgo.GatewayIntentMessageContent,
		discordgo.GatewayIntentGuildScheduledEvents,
		discordgo.GatewayIntentAutomoderationExecution,
		discordgo.GatewayIntentAutomoderationConfiguration,
	}
)

var (
	// the total amount of shards this bot is set to use across all processes
	totalShardCount int
)

// Run intializes and starts the discord bot component of yagpdb
func Run(nodeID string) {
	setup()

	logger.Println("Running bot")

	// either start standalone or set up a connection to a shard orchestrator
	orcheStratorAddress := confShardOrchestratorAddress.GetString()
	if orcheStratorAddress != "" {
		UsingOrchestrator = true
		logger.Infof("Set to use orchestrator at address: %s", orcheStratorAddress)
	} else {
		logger.Info("Running standalone without any orchestrator")
		setupStandalone()
	}

	go mergedMessageSender()

	Running = true

	if UsingOrchestrator {
		NodeConn = node.NewNodeConn(&NodeImpl{}, orcheStratorAddress, common.VERSION, nodeID, nil)
		NodeConn.Run()
	} else {
		ShardManager.Init()
		if usingFixedSharding {
			go ShardManager.Session(fixedShardingID).Open()
		} else {
			go ShardManager.Start()
		}
		botReady()
	}
}

func setup() {
	common.InitSchemas("core_bot", DBSchema)

	discordgo.IdentifyRatelimiter = &identifyRatelimiter{}

	addBotHandlers()
	setupShardManager()
}

func setupStandalone() {
	if confFixedShardingConfig.GetString() == "" {
		shardCount, err := ShardManager.GetRecommendedCount()
		if err != nil {
			panic("Failed getting shard count: " + err.Error())
		}
		totalShardCount = shardCount
	} else {
		fixedShardingID, totalShardCount = readFixedShardingConfig()
		usingFixedSharding = true
		ShardManager.SetNumShards(totalShardCount)
	}
	setupState()

	EventLogger.init(totalShardCount)
	eventsystem.InitWorkers(totalShardCount)
	ReadyTracker.initTotalShardCount(totalShardCount)

	go EventLogger.run()

	for i := 0; i < totalShardCount; i++ {
		ReadyTracker.shardsAdded(i)
	}

	err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", "yagpdb_total_shards", totalShardCount))
	if err != nil {
		logger.WithError(err).Error("failed setting shard count")
	}
}

func readFixedShardingConfig() (id int, count int) {
	conf := confFixedShardingConfig.GetString()
	if conf == "" {
		return 0, 0
	}

	split := strings.SplitN(conf, ",", 2)
	if len(split) < 2 {
		panic("Invalid yagpdb.sharding.fixed_config: " + conf)
	}

	parsedID, err := strconv.ParseInt(split[0], 10, 64)
	if err != nil {
		panic("Invalid yagpdb.sharding.fixed_config: " + err.Error())
	}

	parsedCount, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		panic("Invalid yagpdb.sharding.fixed_config: " + err.Error())
	}

	return int(parsedID), int(parsedCount)
}

// called when the bot is ready and the shards have started connecting
func botReady() {
	pubsub.AddHandler("bot_status_changed", func(evt *pubsub.Event) {
		updateAllShardStatuses()
	}, nil)

	memberFetcher = shardmemberfetcher.NewManager(int64(totalShardCount), State, func(guildID int64, userIDs []int64, nonce string) error {
		shardID := guildShardID(guildID)
		session := ShardManager.Session(shardID)
		if session != nil {
			session.GatewayManager.RequestGuildMembersComplex(&discordgo.RequestGuildMembersData{
				GuildID:   guildID,
				Presences: false,
				UserIDs:   userIDs,
				Nonce:     nonce,
			})
		} else {
			return errors.New("session not found")
		}

		return nil
	}, ReadyTracker)

	serviceDetails := "Not using orchestrator"
	if UsingOrchestrator {
		serviceDetails = "Using orchestrator, NodeID: " + common.NodeID
	}

	// register us with the service discovery
	common.ServiceTracker.RegisterService(common.ServiceTypeBot, "Bot", serviceDetails, botServiceDetailsF)

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

	go runUpdateMetrics()
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

	for i := 0; i < numShards; i++ {
		guilds := State.GetShardGuilds(int64(i))
		result[i] = len(guilds)
	}

	return result
}

type identifyRatelimiter struct {
	mu                   sync.Mutex
	lastShardRatelimited int
	lastRatelimitAt      time.Time
}

var identifyMaxConcurrency int
var identifyConcurrencyOnce sync.Once

func (rl *identifyRatelimiter) getIdentifyMaxConcurrency() int {
	identifyConcurrencyOnce.Do(func() {
		identifyMaxConcurrency = 0
		const redisKey = "yagpdb.gateway.identify.max_concurrency"
		lockKey := redisKey + ":lock"
		err := common.BlockingLockRedisKey(lockKey, 0, 30)
		if err != nil {
			logger.WithError(err).Warn("failed to acquire lock for fetching gateway bot info")
			return
		}
		defer common.UnlockRedisKey(lockKey)
		var cached string
		if err := common.RedisPool.Do(radix.Cmd(&cached, "GET", redisKey)); err == nil && cached != "" {
			if parsed, perr := strconv.Atoi(cached); perr == nil && parsed > 0 {
				identifyMaxConcurrency = parsed
				logger.Infof("Gateway identify max_concurrency (cached): %d", identifyMaxConcurrency)
				return
			}
		}
		s, err := discordgo.New(common.GetBotToken())
		if err != nil {
			logger.WithError(err).Warn("failed to create session to fetch gateway bot info")
			return
		}
		resp, err := s.GatewayBot()
		if err != nil {
			logger.WithError(err).Warn("failed to fetch gateway bot info")
			return
		}
		if resp != nil && resp.SessionStartLimit.MaxConcurrency > 0 {
			identifyMaxConcurrency = resp.SessionStartLimit.MaxConcurrency
		}
		const ttlSeconds = 21600 // cache for 6 hours, this doesn't change often.
		_ = common.RedisPool.Do(radix.FlatCmd(nil, "SETEX", redisKey, ttlSeconds, strconv.Itoa(identifyMaxConcurrency)))
		logger.Infof("Gateway identify max_concurrency: %d", identifyMaxConcurrency)
	})
	return identifyMaxConcurrency
}

func (rl *identifyRatelimiter) RatelimitIdentify(shardID int) {
	mc := 0
	for mc == 0 {
		mc = rl.getIdentifyMaxConcurrency()
		if mc == 0 {
			time.Sleep(5 * time.Second)
		}
	}
	// total buckets is equal to the value of max_concurrency.
	bucket := shardID % mc
	key := "yagpdb.gateway.identify.bucket." + strconv.Itoa(bucket)

	for {
		var resp string
		//instead of a global ratelimit of 1 identify per 5 seconds, we have a bucket ratelimit of 1 identify per 5 seconds per bucket.
		err := common.RedisPool.Do(radix.Cmd(&resp, "SET", key, "1", "PX", "5100", "NX"))
		if err != nil {
			logger.WithError(err).Errorf("failed ratelimiting gateway for bucket %d", bucket)
			time.Sleep(time.Second)
			continue
		}

		if resp == "OK" {
			// We ackquired the lock, our turn to identify now
			rl.mu.Lock()
			rl.lastShardRatelimited = shardID
			rl.lastRatelimitAt = time.Now()
			rl.mu.Unlock()
			return
		}

		// otherwise a identify was sent by someone on this bucket else last 5 seconds
		time.Sleep(time.Second)
	}
}

var confStateRemoveOfflineMembers = config.RegisterOption("yagpdb.state.remove_offline_members", "Remove offline members from state", true)

var StateLimitsF func(guildID int64) (int, time.Duration) = func(guildID int64) (int, time.Duration) {
	return 1000, time.Hour
}

func setupState() {

	removeMembersDur := time.Duration(0)
	if confStateRemoveOfflineMembers.GetBool() {
		removeMembersDur = time.Hour
	}

	tracker := inmemorytracker.NewInMemoryTracker(inmemorytracker.TrackerConfig{
		ChannelMessageLimitsF:     StateLimitsF,
		RemoveOfflineMembersAfter: removeMembersDur,
		BotMemberID:               common.BotUser.ID,
	}, int64(totalShardCount))

	go tracker.RunGCLoop(time.Second)

	eventsystem.DiscordState = tracker

	stateTracker = tracker
	State = tracker
}

func setupShardManager() {
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
		session.Intents = gatewayIntentsUsed

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
}

func botServiceDetailsF() (details *common.BotServiceDetails, err error) {
	if !UsingOrchestrator {
		totalShards := ShardManager.GetNumShards()
		shards := make([]int, totalShards)
		for i := 0; i < totalShards; i++ {
			shards[i] = i
		}

		return &common.BotServiceDetails{
			OrchestratorMode: false,
			TotalShards:      totalShards,
			RunningShards:    shards,
		}, nil
	}

	totalShards := getTotalShards()
	running := ReadyTracker.GetProcessShards()

	return &common.BotServiceDetails{
		TotalShards:      int(totalShards),
		RunningShards:    running,
		NodeID:           common.NodeID,
		OrchestratorMode: true,
	}, nil
}
