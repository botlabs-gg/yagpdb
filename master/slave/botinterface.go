package slave

type Bot interface {
	// Called when a soft start is initiated, make sure to send a EvtSoftStartComplete when completed
	SoftStart()
	// Called when a full start is initiated, either after softstart or immediately (in case were doing a cold start)
	FullStart()

	// Caled when the bot should shut down, make sure to send EvtShutdown when completed
	Shutdown()

	StopShard()
	StartShard()
}
