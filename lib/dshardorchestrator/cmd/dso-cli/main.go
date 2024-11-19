package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator/rest"
	"github.com/jedib0t/go-pretty/table"
	"github.com/mitchellh/cli"
)

var restClient *rest.Client
var serverAddr = os.Getenv("DSO_CLI_ADDR")

func init() {
	// flag.StringVar(&serverAddr, "serveraddr", "http://127.0.0.1:7448", "the rest server address")
}

func main() {
	flag.Parse()
	restClient = rest.NewClient(serverAddr)

	if serverAddr == "" {
		serverAddr = "http://127.0.0.1:7448"
	}

	app := cli.NewCLI("dshardorchestrator-cli", "0.1")
	app.Args = os.Args[1:]

	app.Commands = map[string]cli.CommandFactory{
		"status":        StaticFactory(&StatusCommand{}),
		"startnode":     StaticFactory(&StartNodeCmd{}),
		"shutdownnode":  StaticFactory(&ShutdownNodeCmd{}),
		"migrateshard":  StaticFactory(&MigrateShardCmd{}),
		"migratenode":   StaticFactory(&MigrateNodeCmd{}),
		"fullmigration": StaticFactory(&FullMigrationCmd{}),
		"stopshard":     StaticFactory(&StopShardCmd{}),
		"blacklistnode": StaticFactory(&BlacklistNodeCmd{}),
	}

	exitStatus, err := app.Run()
	if err != nil {
		fmt.Println("Error: ", err)
	}

	os.Exit(exitStatus)
}

type StatusCommand struct{}

func (s *StatusCommand) Help() string {
	return s.Synopsis()
}

func (s *StatusCommand) Run(args []string) int {
	status, err := restClient.GetStatus()
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	tb := table.NewWriter()
	tb.AppendHeader(table.Row{"id", "version", "connected", "shards", "extra"})

	for _, n := range status.Nodes {

		rowExtra := ""

		if n.MigratingFrom != "" {
			rowExtra = fmt.Sprintf("migrating %3d from %s", n.MigratingShard, n.MigratingFrom)
		} else if n.MigratingTo != "" {
			rowExtra = fmt.Sprintf("migrating %3d to   %s", n.MigratingShard, n.MigratingTo)
		}

		if n.Blacklisted {
			if rowExtra != "" {
				rowExtra += ", "
			}
			rowExtra += "blacklisted"
		}

		tb.AppendRow(table.Row{n.ID, n.Version, n.Connected, PrettyFormatNumberList(n.Shards), rowExtra})
	}

	fmt.Println(tb.Render())
	return 0
}

func (s *StatusCommand) Synopsis() string {
	return "display status of all nodes"
}

type StartNodeCmd struct{}

func (s *StartNodeCmd) Help() string {
	return s.Synopsis()
}

func (s *StartNodeCmd) Run(args []string) int {
	msg, err := restClient.StartNewNode()
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *StartNodeCmd) Synopsis() string {
	return "starts a new node"
}

type ShutdownNodeCmd struct{}

func (s *ShutdownNodeCmd) Help() string {
	return s.Synopsis()
}

func (s *ShutdownNodeCmd) Run(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("no node specified")
		return 1
	}

	fmt.Println("shutting down " + args[0])
	msg, err := restClient.ShutdownNode(args[0])
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *ShutdownNodeCmd) Synopsis() string {
	return "shuts down the specified node"
}

type MigrateNodeCmd struct{}

func (s *MigrateNodeCmd) Help() string {
	return s.Synopsis()
}

func (s *MigrateNodeCmd) Run(args []string) int {
	if len(args) < 2 {
		fmt.Println("usage: migratenode origin-node-id target-node-id")
		return 1
	}

	origin := args[0]
	target := args[1]

	fmt.Printf("migrating all shards on %s to %s, this migght take a while....\n", origin, target)

	msg, err := restClient.MigrateNode(origin, target, false)
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *MigrateNodeCmd) Synopsis() string {
	return "migrates origin node to destination node, usage: origin-id destination-id"
}

type MigrateShardCmd struct{}

func (s *MigrateShardCmd) Help() string {
	return s.Synopsis()
}

func (s *MigrateShardCmd) Run(args []string) int {
	if len(args) < 2 {
		fmt.Println("usage: migrateshard shard-id node-id")
		return 1
	}

	shardIDStr := args[0]
	targetNode := args[1]

	shardID, err := strconv.ParseInt(shardIDStr, 10, 32)
	if err != nil {
		fmt.Println("invalid shard: ", err)
		return 1
	}

	fmt.Printf("migrating shard %d to %s...\n", shardID, targetNode)

	msg, err := restClient.MigrateShard(targetNode, int(shardID))
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *MigrateShardCmd) Synopsis() string {
	return "migrates the specified shard to the specified node, usage: shard-id node-id"
}

type FullMigrationCmd struct{}

func (s *FullMigrationCmd) Help() string {
	return s.Synopsis()
}

func (s *FullMigrationCmd) Run(args []string) int {
	fmt.Println("migration all nodes to new nodes, this might take a while")

	msg, err := restClient.MigrateAllNodesToNewNodes()
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *FullMigrationCmd) Synopsis() string {
	return "migrates all shards on all nodes to new nodes"
}

func PrettyFormatNumberList(numbers []int) string {
	if len(numbers) < 1 {
		return "None"
	}

	sort.Ints(numbers)

	var out []string

	last := 0
	seqStart := 0
	for i, n := range numbers {
		if i == 0 {
			last = n
			seqStart = n
			continue
		}

		if n > last+1 {
			// break in sequence
			if seqStart != last {
				out = append(out, fmt.Sprintf("%d - %d", seqStart, last))
			} else {
				out = append(out, fmt.Sprintf("%d", last))
			}

			seqStart = n
		}

		last = n
	}

	if seqStart != last {
		out = append(out, fmt.Sprintf("%d - %d", seqStart, last))
	} else {
		out = append(out, fmt.Sprintf("%d", last))
	}

	return strings.Join(out, ", ")
}

type StopShardCmd struct{}

func (s *StopShardCmd) Help() string {
	return s.Synopsis()
}

func (s *StopShardCmd) Run(args []string) int {
	if len(args) < 1 {
		fmt.Println("usage: stopshard shard-id")
		return 1
	}

	shardIDStr := args[0]

	shardID, err := strconv.ParseInt(shardIDStr, 10, 32)
	if err != nil {
		fmt.Println("invalid shard: ", err)
		return 1
	}

	fmt.Printf("stopping shard %d\n", shardID)

	msg, err := restClient.StopShard(int(shardID))
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *StopShardCmd) Synopsis() string {
	return "stops the specified shard"
}

type BlacklistNodeCmd struct{}

func (s *BlacklistNodeCmd) Help() string {
	return s.Synopsis()
}

func (s *BlacklistNodeCmd) Run(args []string) int {
	if len(args) < 1 {
		fmt.Println("usage: blacklistnode node-id")
		return 1
	}

	nodeID := args[0]

	fmt.Printf("denied the node %s\n", nodeID)

	msg, err := restClient.BlacklistNode(nodeID)
	if err != nil {
		fmt.Println("Error: ", err)
		return 1
	}

	fmt.Println(msg)
	return 0
}

func (s *BlacklistNodeCmd) Synopsis() string {
	return "denies the specified node from being assigned new shards"
}

func StaticFactory(c cli.Command) cli.CommandFactory {
	return func() (cli.Command, error) {
		return c, nil
	}
}
