package node

import (
	"github.com/jonas747/dshardorchestrator/v3"
)

type SessionInfo struct {
	TotalShards int
}

type Interface interface {
	SessionEstablished(info SessionInfo)

	// StopShard should disconnect the specific shard, and return the session info for resumes
	StopShard(shard int) (sessionID string, sequence int64)

	// ResumeShard should resume the speficic shard
	ResumeShard(shard int, sessionID string, sequence int64)

	// AddNewShards should add the following new shards to this node, doing the complete identify flow
	AddNewShards(shards ...int)

	// Caled when the bot should shut down, make sure to send EvtShutdown when completed
	Shutdown()

	// InitializeShardTransferFrom should prepare the shard for a outgoing transfer to another node, disconnecting it and returning the session info
	InitializeShardTransferFrom(shard int) (sessionID string, sequence int64)

	// InitializeShardTransferTo should prepare for a incoming shard transfer, ResumeShard is called aswell so this is not needed in most cases
	InitializeShardTransferTo(shard int, sessionID string, sequence int64)

	// StartShardTransferFrom should return when all user events has been sent, with the number of user events sent
	StartShardTransferFrom(shard int) (numEventsSent int)

	// HandleUserEvent should handle a user event, most commonly used for migrating data between shards during transfers
	HandleUserEvent(evt dshardorchestrator.EventType, data interface{})
}
