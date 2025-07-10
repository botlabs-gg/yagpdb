Sharding orchestrator, see github.com/jonas747/dshardorchestrator for more details.

Stores the total shard number in redis in the key: dshardorchestrator_totalshards

## Usage

To use shardorchestrator in your YAGPDB Instance, you need to do the following things:

### Clone the repository and edit the file

Clone the repository if you haven't already:
```bash
git clone https://github.com/botlabs-gg/yagpdb.git
```

Navigate to the `cmd/shardorchestrator` directory:
```bash
cd yagpdb/cmd/shardorchestrator
```

Edit the `main.go` file on line 77 to match your Node IP:
Example:
```go
	err = orch.Start("192.168.178.5:7447")
```

### Build the binary:
```bash
go build
```

### Start the orchestrator

Export all the required environment variables
```bash
# This is how many shards will be in total
export YAGPDB_SHARDING_TOTAL_SHARDS=

# This is how many shards will be on this node (starts with 0 (e.g. 0-3)
export YAGPDB_SHARDING_ACTIVE_SHARDS=

export YAGPDB_BOTREST_LISTEN_ADDRESS=IP_OF_NODE
```

Finally, start the orchestrator:

```bash
./shardorchestrator
```

### Go to your YAGPDB instance directory

### Configure your YAGPDB instance to use the orchestrator

You need to use 3 more "exports" when starting YAGPDB:
```bash
# These Variables need to match exactly with what you provided via "export" to the shardorchestrator
export YAGPDB_SHARDING_TOTAL_SHARDS=
export YAGPDB_SHARDING_ACTIVE_SHARDS=

export YAGPDB_ORCHESTRATOR_ADDRESS="YOUR_NODE_IP:7447"
```

### Start YAGPDB

When starting YAGPDB in orchestrator mode, you'll also need to provide a node ID for every node.

```bash
./yagpdb -all -nodeid=YOUR_NODE_ID
```