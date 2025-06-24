Sharding orchestrator, see github.com/jonas747/dshardorchestrator for more details.

Stores the total shard number in redis in the key: dshardorchestrator_totalshards

## Usage

To use shardorchestrator in your YAGPDB Instance, you need to do the following things:

### Build the shardorchestrator binary

Clone the repository if you haven't already:
```bash
git clone https://github.com/botlabs-gg/yagpdb.git
```

Go to the shardorchestrator directory and build the binary:
```bash
cd yagpdb/cmd/orchestrator
go build -o orchestrator .
```

Export all the required environment variables
```bash
# This is how many shards will be in total
export YAGPDB_SHARDING_TOTAL_SHARDS=

# This is how many shards will be on this node (starts with 0 (e.g. 0-3)
export YAGPDB_SHARDING_ACTIVE_SHARDS=

export YAGPDB_BOTREST_LISTEN_ADDRESS=IP_OF_NODE

# These should match the values you provided in the YAGPDB instance
export YAGPDB_OWNER=
export YAGPDB_CLIENTID=
export YAGPDB_CLIENTSECRET=
export YAGPDB_BOTTOKEN=
export YAGPDB_REDIS=
export YAGPDB_HOST=
```

### Run the shardorchestrator binary

```bash
# This will run in your console, meaning if you close it it will exit the process
./orchestrator
# This will run in the background
nohup /path/to/your/orchestrator/file &
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
When starting YAGPDB, you also need to provide one more flag
```flag
-nodeid= # You can set this to whatever you want, it is used to identify the node
```

So in the end:
```bash
./yagpdb -all -nodeid=YOUR_NODE_ID
```