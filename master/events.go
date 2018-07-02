package master

// Shard rescaling graceful restarts:
// 1. new slave connects
// 2. master sends the new slave EvtSoftStart
// 		This will make the slave start all the shards, but only process the events in the state handler
// 3. the master waits for EvtSoftStartComplete from the new slave
// 4. the master sends the old slave EvtShutdown and the new slave EvtFullStart

// Shard migration using resumes
// 1. new slave connects
// 2. for each shard
// 		a. master sends EvtStopShard to the old slave
//      b. master waits for EvtShardStopped that includes shard info and state info
//      c. master sends EvtResume to the new slave with the new info
// 3. once out of shards the old slave exits and the new slave starts fully

const (
	// Master -> slave
	EvtSoftStart uint32 = 1 // Sent to signal the slave to not start anything other than start updating the state
	EvtFullStart uint32 = 2 // Sent after a soft start event to start up everything other than the state

	EvtShardMigrationStart uint32 = 3 // Sent to tell a shard to stop all background processing
	EvtStopShard           uint32 = 4 // Sent to tell the slave to stop a shard
	EvtResume              uint32 = 5 // Sent to tell the slave to resume the specified shard

	// Common, sent by both master and slaves
	EvtShutdown uint32 = 6 // Sent to tell a slave to shut down, and immediately stop processing events, responds with the same event once shut down

	// Slave -> master
	EvtSlaveHello        uint32 = 7
	EvtSoftStartComplete uint32 = 8 // Sent to indicate that all shards has been connected and are waiting for the full start event
	EvtShardStopped      uint32 = 9 // Send by a slave when the shard has been stopped, includes state information for guilds related to that shardd
	EvtGuildState        uint32 = 10
)

type SlaveHelloData struct {
	Running bool // Wether the slave was already running or not
}

var EvtDataMap = map[uint32]func() interface{}{
	EvtSlaveHello: func() interface{} { return new(SlaveHelloData) },
}
