package orchestrator

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// StdShardCountProvider is a standard implementation of RecommendedShardCountProvider
type StdShardCountProvider struct {
	DiscordSession *discordgo.Session
}

func (sc *StdShardCountProvider) GetTotalShardCount() (int, error) {
	gwBot, err := sc.DiscordSession.GatewayBot()
	if err != nil {
		return 0, err
	}

	return gwBot.Shards, nil
}

type IDGenerator interface {
	GenerateID() (string, error)
}

type StdNodeLauncher struct {
	IDGenerator   IDGenerator
	LaunchCmdName string
	LaunchArgs    []string

	VersionCmdName string
	VersionArgs    []string

	mu                   sync.Mutex
	lastTimeLaunchedNode time.Time
}

func NewNodeLauncher(cmdName string, args []string, idGen IDGenerator) NodeLauncher {
	return &StdNodeLauncher{
		IDGenerator:   idGen,
		LaunchCmdName: cmdName,
		LaunchArgs:    args,
	}
}

// LaunchNewNode implements NodeLauncher.LaunchNewNode
func (nl *StdNodeLauncher) LaunchNewNode() (string, error) {
	// ensure were not starting nodes too fast since the id generation only does millisecond unique ids
	nl.mu.Lock()
	if time.Since(nl.lastTimeLaunchedNode) < time.Millisecond*100 {
		time.Sleep(time.Millisecond * 100)
	}
	nl.lastTimeLaunchedNode = time.Now()
	nl.mu.Unlock()

	// generate the node id
	var err error
	id := ""
	if nl.IDGenerator != nil {
		id, err = nl.IDGenerator.GenerateID()
	} else {
		id = nl.GenerateID()
	}

	if err != nil {
		return "", err
	}

	args := append(nl.LaunchArgs, "-nodeid", id)

	// launch it
	cmd := exec.Command(nl.LaunchCmdName, args...)
	cmd.Env = os.Environ()

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd.Dir = wd

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	go nl.PrintOutput(stdOut)

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	go nl.PrintOutput(stdErr)

	err = cmd.Start()
	return id, err
}

func (nl *StdNodeLauncher) GenerateID() string {
	tms := time.Now().UnixNano() / 1000000
	st := strconv.FormatInt(tms, 36)

	host, _ := os.Hostname()

	return fmt.Sprintf("%s-%s", host, st)
}

func (nl *StdNodeLauncher) PrintOutput(reader io.Reader) {
	breader := bufio.NewReader(reader)
	for {
		s, err := breader.ReadString('\n')
		if len(s) > 0 {
			s = s[:len(s)-1]
		}

		fmt.Println("NODE: " + s)

		if err != nil {
			break
		}
	}
}

// LaunchVersion implements NodeLauncher.LaunchVersion
func (nl *StdNodeLauncher) LaunchVersion() (string, error) {

	// launch it
	cmd := exec.Command(nl.VersionCmdName, nl.VersionArgs...)
	cmd.Env = os.Environ()

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd.Dir = wd

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
