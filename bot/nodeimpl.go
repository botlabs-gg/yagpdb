package bot

import (
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/mediocregopher/radix/v3"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

func init() {
	dshardorchestrator.RegisterUserEvent("GuildState", EvtGuildState, dstate.GuildSet{})
	dshardorchestrator.RegisterUserEvent("MemberState", EvtMember, dstate.MemberState{})
}

// Implementation of DShardOrchestrator/Node/Interface
var _ node.Interface = (*NodeImpl)(nil)

type NodeImpl struct {
	lastTimeFreedMemory   time.Time
	lastTimeFreedMemorymu sync.Mutex
}

func (n *NodeImpl) SessionEstablished(info node.SessionInfo) {
	if info.TotalShards == 0 {
		panic("got total shard count of 0?!!?")
	}

	if totalShardCount == 0 {
		totalShardCount = info.TotalShards
		setupState()

		ShardManager.SetNumShards(totalShardCount)
		eventsystem.InitWorkers(totalShardCount)
		ReadyTracker.initTotalShardCount(totalShardCount)

		EventLogger.init(info.TotalShards)
		go EventLogger.run()

		err := common.RedisPool.Do(radix.FlatCmd(nil, "SET", "yagpdb_total_shards", info.TotalShards))
		if err != nil {
			logger.WithError(err).Error("failed setting shard count")
		}

		err = ShardManager.Init()
		if err != nil {
			panic("failed initializing discord sessions: " + err.Error())
		}

		botReady()
	}
}

func (n *NodeImpl) StopShard(shard int) (sessionID string, sequence int64, resumeGatewayUrl string) {
	ReadyTracker.shardRemoved(shard)

	n.lastTimeFreedMemorymu.Lock()
	freeMem := false
	if time.Since(n.lastTimeFreedMemory) > time.Minute {
		freeMem = true
		n.lastTimeFreedMemory = time.Now()
	}
	n.lastTimeFreedMemorymu.Unlock()

	if freeMem {
		debug.FreeOSMemory()
	}

	err := ShardManager.Sessions[shard].Close()
	if err != nil {
		logger.WithError(err).Error("failed stopping shard: ", err)
	}

	sessionID, sequence, resumeGatewayUrl = ShardManager.Sessions[shard].GatewayManager.GetSessionInfo()
	return
}

func (n *NodeImpl) ResumeShard(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
	ReadyTracker.shardsAdded(shard)

	ShardManager.Sessions[shard].GatewayManager.SetSessionInfo(sessionID, sequence, resumeGatewayUrl)
	err := ShardManager.Sessions[shard].GatewayManager.Open()
	if err != nil {
		logger.WithError(err).Error("Failed migrating shard")
	}
}

func (n *NodeImpl) AddNewShards(shards ...int) {
	ReadyTracker.shardsAdded(shards...)

	for _, shard := range shards {
		ShardManager.Sessions[shard].GatewayManager.SetSessionInfo("", 0, "")

		go ShardManager.Sessions[shard].GatewayManager.Open()
	}

	logger.Infof("got assigned shards %v", shards)
}

// called when the bot should shut down, make sure to send EvtShutdown when completed
func (n *NodeImpl) Shutdown() {
	var wg sync.WaitGroup
	Stop(&wg)
	wg.Wait()

	os.Exit(0)
}

func (n *NodeImpl) InitializeShardTransferFrom(shard int) (sessionID string, sequence int64, resumeGatewayUrl string) {
	return n.StopShard(shard)
}

func (n *NodeImpl) InitializeShardTransferTo(shard int, sessionID string, sequence int64, resumeGatewayUrl string) {
	// this isn't actually needed, as startshard will be called with the same session details
}

const (
	// This was a legacy format thats now unused
	// EvtGuildState dshardorchestrator.EventType = 101

	EvtGuildState dshardorchestrator.EventType = 102
	EvtMember     dshardorchestrator.EventType = 103
)

// this should return when all user events has been sent, with the number of user events sent
func (n *NodeImpl) StartShardTransferFrom(shard int) (numEventsSent int) {
	return n.SendGuilds(shard)
}

func (n *NodeImpl) HandleUserEvent(evt dshardorchestrator.EventType, data interface{}) {

	if evt == EvtGuildState {
		dataCast := data.(*dstate.GuildSet)
		stateTracker.SetGuild(dataCast)
	} else if evt == EvtMember {
		dataCast := data.(*dstate.MemberState)
		stateTracker.SetMember(dataCast)
	}

	for _, v := range common.Plugins {
		if migrator, ok := v.(ShardMigrationReceiver); ok {
			migrator.ShardMigrationReceive(evt, data)
		}
	}
}

func (n *NodeImpl) SendGuilds(shard int) int {
	started := time.Now()

	pluginSentEvents := 0

	// start with the plugins
	for _, v := range common.Plugins {
		if migrator, ok := v.(ShardMigrationSender); ok {
			pluginSentEvents += migrator.ShardMigrationSend(shard)
		}
	}

	// Send the guilds on this shard
	guildsToSend := State.GetShardGuilds(int64(shard))

	workChan := make(chan *dstate.GuildSet)
	var wg sync.WaitGroup

	sentEvents := new(int32)

	// To speed this up we use multiple workers, this has to be done in a relatively short timespan otherwise we won't be able to resume
	worker := func() {
		for gs := range workChan {
			NodeConn.SendLogErr(EvtGuildState, gs, true)
			if ms := State.GetMember(gs.ID, common.BotUser.ID); ms != nil {
				NodeConn.SendLogErr(EvtMember, ms, true)
				atomic.AddInt32(sentEvents, 2)
			} else {
				atomic.AddInt32(sentEvents, 1)
			}
		}

		wg.Done()
	}

	// spawn runtime.NumCPU - 2 workers
	numCpu := runtime.NumCPU()
	numWorkers := numCpu - 2
	if numWorkers < 2 {
		numWorkers = 2
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	for _, v := range guildsToSend {
		workChan <- v
	}
	close(workChan)
	wg.Wait()

	// clean up after ourselves
	stateTracker.DelShard(int64(shard))

	logger.Printf("Took %s to transfer %d objects", time.Since(started), atomic.LoadInt32(sentEvents))
	return int(atomic.LoadInt32(sentEvents)) + pluginSentEvents
}
