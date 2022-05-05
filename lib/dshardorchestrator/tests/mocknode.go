package tests

import (
	"strconv"
	"sync/atomic"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/node"
)

type MockBot struct {
	SessionEstablishedFunc func(info node.SessionInfo)

	StopShardFunc   func(shard int) (sessionID string, sequence int64)
	ResumeShardFunc func(shard int, sessionID string, sequence int64)
	AddNewShardFunc func(shards ...int)

	// Caled when the bot should shut down, make sure to send EvtShutdown when completed
	ShutdownFunc func()

	InitializeShardTransferFromFunc func(shard int) (sessionID string, sequence int64)
	InitializeShardTransferToFunc   func(shard int, sessionID string, sequence int64)

	// this should return when all user events has been sent, with the number of user events sent
	StartShardTransferFromFunc func(shard int) (numEventsSent int)

	HandleUserEventFunc func(evt dshardorchestrator.EventType, data interface{})
}

var _ node.Interface = (*MockBot)(nil)

func (mn *MockBot) SessionEstablished(info node.SessionInfo) {
	if mn.SessionEstablishedFunc != nil {
		mn.SessionEstablishedFunc(info)
	}
}

func (mn *MockBot) StopShard(shard int) (sessionID string, sequence int64) {
	if mn.StopShardFunc != nil {
		return mn.StopShardFunc(shard)
	}
	return "", 0
}

func (mn *MockBot) AddNewShards(shards ...int) {
	if mn.AddNewShardFunc != nil {
		mn.AddNewShardFunc(shards...)
	}
}

func (mn *MockBot) ResumeShard(shard int, sessionID string, sequence int64) {
	if mn.ResumeShardFunc != nil {
		mn.ResumeShardFunc(shard, sessionID, sequence)
	}
}

// Caled when the bot should shut down, make sure to send EvtShutdown when completed
func (mn *MockBot) Shutdown() {
	if mn.ShutdownFunc != nil {
		mn.ShutdownFunc()
	}
}

func (mn *MockBot) InitializeShardTransferFrom(shard int) (sessionID string, sequence int64) {
	if mn.InitializeShardTransferFromFunc != nil {
		return mn.InitializeShardTransferFromFunc(shard)
	}

	return "", 0
}

func (mn *MockBot) InitializeShardTransferTo(shard int, sessionID string, sequence int64) {
	if mn.InitializeShardTransferToFunc != nil {
		mn.InitializeShardTransferToFunc(shard, sessionID, sequence)
	}

}

// this should return when all user events has been sent, with the number of user events sent
func (mn *MockBot) StartShardTransferFrom(shard int) (numEventsSent int) {
	if mn.StartShardTransferFromFunc != nil {
		return mn.StartShardTransferFromFunc(shard)
	}

	return 0
}

func (mn *MockBot) HandleUserEvent(evt dshardorchestrator.EventType, data interface{}) {
	if mn.HandleUserEventFunc != nil {
		mn.HandleUserEventFunc(evt, data)
	}
}

var lastID = new(int32)

func generateID() string {
	newID := atomic.AddInt32(lastID, 1)
	return strconv.Itoa(int(newID)) + "-testing"
}
