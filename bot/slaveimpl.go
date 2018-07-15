package bot

import (
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/master"
	"github.com/jonas747/yagpdb/master/slave"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Slave implementation
type SlaveImpl struct{}

var _ slave.Bot = (*SlaveImpl)(nil)

func (s *SlaveImpl) FullStart() {
	logrus.Println("Starting full start")

	stateLock.Lock()
	stateTmp := state
	state = StateRunningWithMaster
	stateLock.Unlock()

	if stateTmp != StateSoftStarting {
		InitPlugins()
		go ShardManager.Start()
	}
}

func (s *SlaveImpl) SoftStart() {
	logrus.Println("Starting soft start")

	stateLock.Lock()
	state = StateSoftStarting
	stateLock.Unlock()

	InitPlugins()

	go ShardManager.Start()
}

func (s *SlaveImpl) Shutdown() {
	ShardManager.StopAll()
	os.Exit(1)
}

func (s *SlaveImpl) StartShardTransferFrom() int {
	stateLock.Lock()
	state = StateShardMigrationFrom
	stateLock.Unlock()

	var wg sync.WaitGroup
	StopAllPlugins(&wg)
	wg.Wait()

	return ShardManager.GetNumShards()
}

func (s *SlaveImpl) StartShardTransferTo(numShards int) {
	ShardManager.SetNumShards(numShards)

	InitPlugins()

	err := ShardManager.Init()
	if err != nil {
		panic("Failed initializing shard manager: " + err.Error())
	}

	stateLock.Lock()
	state = StateShardMigrationTo
	stateLock.Unlock()

	atomic.StoreInt32(botStartedFired, 1)
}

func (s *SlaveImpl) StopShard(shard int) (sessionID string, sequence int64) {
	err := ShardManager.Sessions[shard].Close()
	if err != nil {
		logrus.WithError(err).Error("Failed stopping shard: ", err)
	}

	// Wait a second to be sure we dont have any event handlers still running populating the state for this shard
	time.Sleep(time.Second)

	s.SendGuilds(shard)

	sessionID, sequence = ShardManager.Sessions[shard].GatewayManager.GetSessionInfo()
	return
}

func (s *SlaveImpl) SendGuilds(shard int) {
	numShards := ShardManager.GetNumShards()

	started := time.Now()

	// Send the guilds on this shard
	guildsToSend := make([]*dstate.GuildState, 0)
	State.RLock()
	for _, v := range State.Guilds {
		shardID := (v.ID >> 22) % int64(numShards)
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

			SlaveClient.Send(master.EvtGuildState, &master.GuildStateData{GuildState: gs}, true)
			gs.RUnlock()

			State.Lock()
			for _, c := range channels {
				delete(State.Channels, c)
			}
			State.Unlock()
		}

		wg.Done()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go worker()
	}

	for _, v := range guildsToSend {
		workChan <- v
	}
	close(workChan)
	wg.Wait()

	runtime.GC()

	logrus.Println("Took ", time.Since(started), " to transfer ", len(guildsToSend), "guildstates")

}

func (s *SlaveImpl) StartShard(shard int, sessionID string, sequence int64) {
	ShardManager.Sessions[shard].GatewayManager.SetSessionInfo(sessionID, sequence)
	err := ShardManager.Sessions[shard].GatewayManager.Open()
	if err != nil {
		logrus.WithError(err).Error("Failed migrating shard")
	}

	numShards := ShardManager.GetNumShards()
	if numShards-1 == shard {
		// Done!
		logrus.Println("shard migration complete!")
		stateLock.Lock()
		state = StateRunningWithMaster
		stateLock.Unlock()

		BotStarted()
	}
}

func (s *SlaveImpl) LoadGuildState(gs *master.GuildStateData) {

	guild := gs.GuildState
	for _, c := range guild.Channels {
		c.Owner = guild
		c.Guild = guild
	}

	for _, m := range guild.Members {
		m.Guild = guild
	}

	State.Lock()
	State.Guilds[guild.ID] = guild
	for _, c := range guild.Channels {
		State.Channels[c.ID] = c
	}
	State.Unlock()

	for _, v := range common.Plugins {
		if guildMigrationHandler, ok := v.(ShardMigrationHandler); ok {
			if ok {
				guildMigrationHandler.GuildMigrated(guild, true)
			}
		}
	}
}
