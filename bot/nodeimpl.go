package bot

import (
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/mediocregopher/radix/v3"

	"github.com/jonas747/dshardorchestrator/v2"
	"github.com/jonas747/dshardorchestrator/v2/node"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
)

func init() {
	dshardorchestrator.RegisterUserEvent("GuildState", EvtGuildState, dstate.GuildState{})
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

func (n *NodeImpl) StopShard(shard int) (sessionID string, sequence int64) {
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

	sessionID, sequence = ShardManager.Sessions[shard].GatewayManager.GetSessionInfo()
	return
}

func (n *NodeImpl) ResumeShard(shard int, sessionID string, sequence int64) {
	ReadyTracker.shardsAdded(shard)

	ShardManager.Sessions[shard].GatewayManager.SetSessionInfo(sessionID, sequence)
	err := ShardManager.Sessions[shard].GatewayManager.Open()
	if err != nil {
		logger.WithError(err).Error("Failed migrating shard")
	}
}

func (n *NodeImpl) AddNewShards(shards ...int) {
	ReadyTracker.shardsAdded(shards...)

	for _, shard := range shards {
		ShardManager.Sessions[shard].GatewayManager.SetSessionInfo("", 0)

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

func (n *NodeImpl) InitializeShardTransferFrom(shard int) (sessionID string, sequence int64) {
	return n.StopShard(shard)
}

func (n *NodeImpl) InitializeShardTransferTo(shard int, sessionID string, sequence int64) {
	// this isn't actually needed, as startshard will be called with the same session details
}

const (
	EvtGuildState dshardorchestrator.EventType = 101
)

// this should return when all user events has been sent, with the number of user events sent
func (n *NodeImpl) StartShardTransferFrom(shard int) (numEventsSent int) {
	return n.SendGuilds(shard)
}

func (n *NodeImpl) HandleUserEvent(evt dshardorchestrator.EventType, data interface{}) {
	if evt == EvtGuildState {
		dataCast := data.(*dstate.GuildState)
		n.LoadGuildState(dataCast)
	}

	for _, v := range common.Plugins {
		if migrator, ok := v.(ShardMigrationReceiver); ok {
			migrator.ShardMigrationReceive(evt, data)
		}
	}
}

func (n *NodeImpl) SendGuilds(shard int) int {
	started := time.Now()

	totalSentEvents := 0
	// start with the plugins
	for _, v := range common.Plugins {
		if migrator, ok := v.(ShardMigrationSender); ok {
			totalSentEvents += migrator.ShardMigrationSend(shard)
		}
	}

	// Send the guilds on this shard
	guildsToSend := make([]*dstate.GuildState, 0)
	State.RLock()
	for _, v := range State.Guilds {
		shardID := guildShardID(v.ID)
		if int(shardID) == shard {
			guildsToSend = append(guildsToSend, v)
		}
	}
	State.RUnlock()

	workChan := make(chan *dstate.GuildState)
	var wg sync.WaitGroup

	// To speed this up we use multiple workers, this has to be done in a relatively short timespan otherwise we won't be able to resume
	worker := func() {
		for gs := range workChan {
			State.Lock()
			delete(State.Guilds, gs.ID)
			State.Unlock()

			gs.RLock()
			channels := make([]int64, 0, len(gs.Channels))
			for _, c := range gs.Channels {
				channels = append(channels, c.ID)
			}

			NodeConn.SendLogErr(EvtGuildState, gs, true)
			gs.RUnlock()

			State.Lock()
			for _, c := range channels {
				delete(State.Channels, c)
			}
			State.Unlock()
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

	logger.Println("Took ", time.Since(started), " to transfer ", len(guildsToSend), "guildstates")
	totalSentEvents += len(guildsToSend)
	return totalSentEvents
}

func (n *NodeImpl) LoadGuildState(gs *dstate.GuildState) {

	for _, c := range gs.Channels {
		c.Owner = gs
		c.Guild = gs
	}

	for _, m := range gs.Members {
		m.Guild = gs
	}

	gs.InitCache(State)

	State.Lock()
	State.Guilds[gs.ID] = gs
	for _, c := range gs.Channels {
		State.Channels[c.ID] = c
	}
	State.Unlock()
}
