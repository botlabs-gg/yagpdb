package dshardorchestrator

type IdentifyData struct {
	TotalShards   int
	RunningShards []int
	Version       string
	NodeID        string

	// when the logic of the orhcestrator changes in a backwards incompatible way, we reject nodes with mistmatched logic versions
	OrchestratorLogicVersion int
}

type IdentifiedData struct {
	TotalShards int
	NodeID      string
}

type StartShardsData struct {
	ShardIDs []int
}

type StopShardData struct {
	ShardID int
}

type PrepareShardmigrationData struct {
	// wether this is the node were migrating the shard from
	Origin  bool
	ShardID int

	SessionID        string
	Sequence         int64
	ResumeGatewayUrl string
}

type StartshardMigrationData struct {
	ShardID int
}

type AllUserDataSentData struct {
	NumEvents int
}
