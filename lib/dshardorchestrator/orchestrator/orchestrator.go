package orchestrator

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
	"github.com/pkg/errors"
)

type NodeIDProvider interface {
	GenerateID() string
}

// RecommendTotalShardCountProvider will only be called when a fresh new shard count is needed
// this is only the case if: no nodes were running with any shards 10 seconds after the orchestrator starts up
//
// if new nodes without shards connect during this period, they will be put on hold while waiting for nodes with
// running shards to re-identify with the orchestrator, thereby keeping the total shard count across restarts of the orchestrator
// in a fairly realiable manner
//
// but using this interface you can implement completely fine grained control (say storing the shard count in persistent store and updating it manually)
type RecommendTotalShardCountProvider interface {
	GetTotalShardCount() (int, error)
}

// NodeLauncher is responsible for the logic of spinning up new processes and also receiving the version of the node that would be launched
type NodeLauncher interface {
	// Launches a new node, returning the id and a error if something went wrong
	LaunchNewNode() (nodeID string, err error)

	// Retrieves the version of nodes we would launch if we were to call LaunchNewNode()
	// for example, the vesion of the binary deployed on the server.
	LaunchVersion() (version string, err error)
}

// VersionUpdater is reponsible for updating the deployment, for example pulling a new version from a CI server
type VersionUpdater interface {
	PullNewVersion() (newVersion string, err error)
}

type Orchestrator struct {
	// these fields are only safe to edit before you start the orchestrator
	// if you decide to change anything afterwards, it may panic or cause undefined behaviour

	ShardCountProvider RecommendTotalShardCountProvider
	NodeLauncher       NodeLauncher
	Logger             dshardorchestrator.Logger
	VersionUpdater     VersionUpdater

	// this is for running in a multi-host mode
	// this allows you to have 1 shard orchestrator per host, then only have that orchestator care about the specified shards
	// this also requires that the total shard count is fixed.
	// FixedTotalShardCount will be ignored if <1
	// ResponsibleForShards will be ignored if len < 1
	FixedTotalShardCount int
	ResponsibleForShards []int

	// if set, the orchestrator will make sure that all the shards are always running
	EnsureAllShardsRunning bool

	// For large bot sharding the bucket size should be 16
	// the orchestrator will only put shards in the same (bucket/bucketspernode) on the same node
	ShardBucketSize int
	// The number of buckets per node, this * shardBucketSize should equal to the actual bots bucket size, but this allows more gradual control of the startup process
	BucketsPerNode int

	// the max amount of downtime for a node before we consider it dead and it will start a new node for those shards
	// if set to below zero then it will not perform the restart at all
	MaxNodeDowntimeBeforeRestart time.Duration

	// the maximum amount of shards per node, note that this is solely for the automated tasks the orchestrator provides
	// and you can still go over it if you manually start shards on a node
	MaxShardsPerNode int

	// in case we are intiailizing max shards from nodes, we wait 10 seconds when we start before we decide we need to fetch a fresh shard count
	SkipSafeStartupDelayMaxShards bool

	monitor *monitor

	// below fields are protected by the following mutex
	mu             sync.Mutex
	connectedNodes []*NodeConn
	totalShards    int

	activeMigrationFrom string
	activeMigrationTo   string
	netListener         net.Listener

	// blacklisted nodes will not get new shards assigned to them
	blacklistedNodes []string

	performingFullMigration bool
}

func NewStandardOrchestrator(session *discordgo.Session) *Orchestrator {
	return &Orchestrator{
		ShardCountProvider: &StdShardCountProvider{DiscordSession: session},
	}
}

// Start will start the orchestrator, and start to listen for clients on the specified address
// IMPORTANT: opening this up to the outer internet is bad because there's no authentication.
func (o *Orchestrator) Start(listenAddr string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	err := o.openListen(listenAddr)
	if err != nil {
		return err
	}

	o.monitor = &monitor{
		orchestrator: o,
		stopChan:     make(chan bool),
	}
	go o.monitor.run()

	return nil
}

// Stop will stop the orchestrator and the monitor
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.netListener.Close()
	o.monitor.stop()
}

// openListen starts listening for slave connections on the specified address
func (o *Orchestrator) openListen(addr string) error {

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.WithMessage(err, "net.Listen")
	}

	o.netListener = listener

	// go monitorSlaves()
	go o.listenForNodes(listener)

	return nil
}

func (o *Orchestrator) listenForNodes(listener net.Listener) {
	o.Log(dshardorchestrator.LogInfo, nil, "listening for incoming nodes on: "+listener.Addr().String())
	for {
		conn, err := listener.Accept()
		if err != nil {
			o.Log(dshardorchestrator.LogError, err, "failed accepting incmoing connection")
			break
		}

		o.Log(dshardorchestrator.LogInfo, nil, "new node connection!")
		client := o.NewNodeConn(conn)

		o.mu.Lock()
		o.connectedNodes = append(o.connectedNodes, client)
		o.mu.Unlock()

		go client.listen()
	}
}

func (o *Orchestrator) FindNodeByID(id string) *NodeConn {
	o.mu.Lock()
	for _, v := range o.connectedNodes {
		if v.Conn.GetID() == id {
			o.mu.Unlock()
			return v
		}
	}
	o.mu.Unlock()

	return nil
}

type NodeStatus struct {
	ID                 string
	Version            string
	SessionEstablished bool
	Shards             []int
	Connected          bool
	DisconnectedAt     time.Time
	Blacklisted        bool

	MigratingFrom  string
	MigratingTo    string
	MigratingShard int
}

// GetFullNodesStatus returns the full status of all nodes
func (o *Orchestrator) GetFullNodesStatus() []*NodeStatus {
	result := make([]*NodeStatus, 0)

	o.mu.Lock()
	for _, v := range o.connectedNodes {
		o.mu.Unlock()
		st := v.GetFullStatus()
		o.mu.Lock()

		st.Blacklisted = o.isNodeBlacklisted(false, st.ID)
		result = append(result, st)
	}
	o.mu.Unlock()

	return result
}

var (
	ErrUnknownFromNode         = errors.New("unknown 'from' node")
	ErrUnknownToNode           = errors.New("unknown 'to' node")
	ErrFromNodeNotRunningShard = errors.New("'from' node not running shard")
	ErrNodeBusy                = errors.New("node is busy")
)

// StartShardMigration attempts to start a shard migration, moving shardID from a origin node to a destination node
func (o *Orchestrator) StartShardMigration(toNodeID string, shardID int) error {

	// find the origin node
	fromNodeID := ""

	status := o.GetFullNodesStatus()
OUTER:
	for _, n := range status {
		if !n.Connected {
			continue
		}

		for _, s := range n.Shards {
			if s == shardID {
				fromNodeID = n.ID
				break OUTER
			}
		}
	}

	toNode := o.FindNodeByID(toNodeID)
	fromNode := o.FindNodeByID(fromNodeID)

	if fromNode == nil {
		return ErrUnknownFromNode
	}

	if toNode == nil {
		return ErrUnknownToNode
	}

	// mark the origin node as busy
	fromNode.mu.Lock()
	// make sure the node were migrating from actually holds the shard
	foundShard := false
	for _, v := range fromNode.runningShards {
		if v == shardID {
			foundShard = true
			break
		}
	}

	// it did not hold the shard
	if !foundShard {
		fromNode.mu.Unlock()
		return ErrFromNodeNotRunningShard
	}

	// make sure its not busy
	if fromNode.shardMigrationMode != dshardorchestrator.ShardMigrationModeNone {
		fromNode.mu.Unlock()
		return ErrNodeBusy
	}

	fromNode.shardMigrationMode = dshardorchestrator.ShardMigrationModeFrom
	fromNode.shardMigrationOtherNodeID = toNodeID
	fromNode.shardMigrationShard = shardID
	// fromNode.shardmigrationTotalUserEvts = 0
	fromNode.mu.Unlock()

	// mark the destination node as busy
	toNode.mu.Lock()

	// make sure its not busy
	if toNode.shardMigrationMode != dshardorchestrator.ShardMigrationModeNone {
		toNode.mu.Unlock()

		fromNode.mu.Lock()
		// need to rollback the from state as we cannot go further
		fromNode.shardMigrationMode = dshardorchestrator.ShardMigrationModeNone
		fromNode.shardMigrationOtherNodeID = ""
		fromNode.shardMigrationShard = -1
		fromNode.mu.Unlock()

		toNode.mu.Unlock()
		return ErrNodeBusy
	}

	toNode.shardMigrationMode = dshardorchestrator.ShardMigrationModeTo
	toNode.shardMigrationOtherNodeID = fromNodeID
	toNode.shardMigrationShard = shardID
	toNode.mu.Unlock()

	o.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("migrating shard %d from %q to %q", shardID, fromNodeID, toNodeID))

	// everything passed, we can start the migration of the shard
	fromNode.Conn.SendLogErr(dshardorchestrator.EvtPrepareShardmigration, &dshardorchestrator.PrepareShardmigrationData{
		Origin:  true,
		ShardID: shardID,
	})

	return nil
}

// Log will log to the designated logger or he standard logger
func (o *Orchestrator) Log(level dshardorchestrator.LogLevel, err error, msg string) {
	if err != nil {
		msg = msg + ": " + err.Error()
	}

	if o.Logger == nil {
		dshardorchestrator.StdLogInstance.Log(level, msg)
	} else {
		o.Logger.Log(level, msg)
	}
}

var (
	ErrNoNodeLauncher = errors.New("orchestrator.NodeLauncher is nil")
)

// StartNewNode will launch a new node, it will not wait for it to connect
func (o *Orchestrator) StartNewNode() (string, error) {
	if o.NodeLauncher == nil {
		return "", ErrNoNodeLauncher
	}

	return o.NodeLauncher.LaunchNewNode()
}

var (
	ErrShardAlreadyRunning = errors.New("shard already running")
	ErrUnknownNode         = errors.New("unknown node")
)

// StartShard will start the specified shard on the specified node
// it will return ErrShardAlreadyRunning if the shard is running on another node already
func (o *Orchestrator) StartShards(nodeID string, shards ...int) error {
	fullStatus := o.GetFullNodesStatus()
	for _, v := range fullStatus {
		if !v.Connected {
			continue
		}

		for _, s := range shards {
			if dshardorchestrator.ContainsInt(v.Shards, s) {
				return ErrShardAlreadyRunning
			}
		}
	}

	node := o.FindNodeByID(nodeID)
	if node == nil {
		return ErrUnknownNode
	}

	node.StartShards(shards...)
	return nil
}

// StopShard will stop the specified shard on whatever node it's running on, or do nothing if it's not running
func (o *Orchestrator) StopShard(shard int) error {
	fullStatus := o.GetFullNodesStatus()
	for _, v := range fullStatus {
		if !v.Connected {
			continue
		}

		if dshardorchestrator.ContainsInt(v.Shards, shard) {
			// bingo
			node := o.FindNodeByID(v.ID)
			if node == nil {
				return ErrUnknownNode
			}

			node.StopShard(shard)
		}
	}

	return nil
}

// MigrateFullNode migrates all the shards on the origin node to the destination node
// optionally also shutting the origin node down at the end
func (o *Orchestrator) MigrateFullNode(fromNode string, toNodeID string, shutdownOldNode bool) error {
	nodeFrom := o.FindNodeByID(fromNode)
	if nodeFrom == nil {
		return ErrUnknownFromNode
	}

	toNode := o.FindNodeByID(toNodeID)
	if toNode == nil {
		return ErrUnknownToNode
	}
	nodeFrom.mu.Lock()
	shards := make([]int, len(nodeFrom.runningShards))
	copy(shards, nodeFrom.runningShards)
	nodeFrom.mu.Unlock()

	o.Log(dshardorchestrator.LogInfo, nil, fmt.Sprintf("starting full node migration from %s to %s, n-shards: %d", fromNode, toNodeID, len(shards)))

	for _, s := range shards {
		err := o.StartShardMigration(toNodeID, s)
		if err != nil {
			return err
		}

		// wait for it to be moved before we start the next one
		o.WaitForShardMigration(nodeFrom, toNode, s)

		// reset here in case something went wrong
		toNode.mu.Lock()
		toNode.shardMigrationMode = dshardorchestrator.ShardMigrationModeNone
		toNode.mu.Unlock()

		nodeFrom.mu.Lock()
		nodeFrom.shardMigrationMode = dshardorchestrator.ShardMigrationModeNone
		nodeFrom.mu.Unlock()

		// wait a bit extra to allow for some time ot catch up on events processing
		time.Sleep(time.Second)
	}

	if shutdownOldNode {
		return o.ShutdownNode(fromNode)
	}

	return nil
}

// ShutdownNode shuts down the specified node
func (o *Orchestrator) ShutdownNode(nodeID string) error {
	node := o.FindNodeByID(nodeID)
	if node == nil {
		return ErrUnknownNode
	}

	node.Shutdown()
	return nil
}

// WaitForShardMigration blocks until a shard migration is complete
func (o *Orchestrator) WaitForShardMigration(fromNode *NodeConn, toNode *NodeConn, shardID int) {
	// wait for the shard to dissapear on the origin node
	for {
		time.Sleep(time.Second)

		status := fromNode.GetFullStatus()

		if !dshardorchestrator.ContainsInt(status.Shards, shardID) || !status.Connected {
			// also if we disconnected then just go through all this immeditely
			break
		}
	}

	// wait for it to appear on the new node
	for {
		time.Sleep(time.Millisecond * 100)

		statusTo := toNode.GetFullStatus()
		statusFrom := fromNode.GetFullStatus()

		if dshardorchestrator.ContainsInt(statusTo.Shards, shardID) || !statusFrom.Connected {
			// also if we disconnected then just go through all this immeditely
			break
		}
	}

	// AND FINALLY just for safe measure, wait for it to not be in the migrating state
	for {
		time.Sleep(time.Millisecond * 100)

		statusTo := toNode.GetFullStatus()
		statusFrom := fromNode.GetFullStatus()

		if statusTo.MigratingFrom == "" || !statusFrom.Connected {
			break
		}
	}
}

func (o *Orchestrator) findAvailableNode(ignore []*NodeStatus) (string, error) {
	numFailed := 0

	var lastTimeStartedNode time.Time

	for {
		// look for a available node
		nodes := o.GetFullNodesStatus()
	FINDOUTER:
		for _, v := range nodes {
			if !v.Connected || len(v.Shards) > 0 {
				continue
			}

			for _, ig := range ignore {
				if ig.ID == v.ID {
					continue FINDOUTER
				}
			}

			if v.MigratingFrom != "" || v.MigratingTo != "" {
				continue
			}

			return v.ID, nil
		}

		// need to start a new node
		// we wait inbetween 10 seconds each launch
		// nodes may have been "stolen" by someone in between so we can retry if we fail after 10 seconds
		// TODO: reserve nodes
		if time.Since(lastTimeStartedNode) < time.Second*60 {
			time.Sleep(time.Second)
			continue
		}

		numFailed++
		if numFailed > 5 {
			return "", errors.New("Failed 5 times")
		}

		_, err := o.StartNewNode()
		if err != nil {
			return "", err
		}

		lastTimeStartedNode = time.Now()
		time.Sleep(time.Second)
	}
}

// MigrateAllNodesToNewNodes performs a full migration of all nodes
// if returnOnError is set then it will return when one of the nodes fail migrating, otherwise it will just log and continue onto the next
func (o *Orchestrator) MigrateAllNodesToNewNodes(returnOnError bool) error {
	o.mu.Lock()
	if o.performingFullMigration {
		o.mu.Unlock()
		return errors.New("Already performing a full migration")
	}
	o.performingFullMigration = true
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.performingFullMigration = false
		o.mu.Unlock()
	}()

	o.Log(dshardorchestrator.LogInfo, nil, "performing a full node migration on all nodes")

	nodes := o.GetFullNodesStatus()

	for _, v := range nodes {
		if !v.Connected || len(v.Shards) < 1 {
			continue
		}

		targetID, err := o.findAvailableNode(nodes)
		if err != nil {
			return err
		}

		err = o.MigrateFullNode(v.ID, targetID, true)
		if err != nil {
			if returnOnError {
				return err
			} else {
				o.Log(dshardorchestrator.LogError, err, fmt.Sprintf("failed migrating %s to %s", v.ID, targetID))
			}
		}

		// sleep a bit to allow for a buffer for things to settle post migration (maybe some reconnects triggered and such...)
		time.Sleep(time.Second * 5)
	}

	return nil
}

func (o *Orchestrator) getTotalShardCount() (int, error) {
	if o.FixedTotalShardCount > 0 {
		return o.FixedTotalShardCount, nil
	}

	return o.ShardCountProvider.GetTotalShardCount()
}

func (o *Orchestrator) isResponsibleForShard(shard int) bool {
	if len(o.ResponsibleForShards) < 1 {
		return true
	}

	for _, v := range o.ResponsibleForShards {
		if v == shard {
			return true
		}
	}

	return false
}

// BlacklistNode blacklists a node from beign designated new shards
func (o *Orchestrator) BlacklistNode(node string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isNodeBlacklisted(false, node) {
		// already blacklisted
		return
	}

	o.blacklistedNodes = append(o.blacklistedNodes, node)
}

func (o *Orchestrator) isNodeBlacklisted(lock bool, node string) bool {
	if lock {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	for _, v := range o.blacklistedNodes {
		if v == node {
			return true
		}
	}

	return false
}
