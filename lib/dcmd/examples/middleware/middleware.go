package main

/*
This example provides provides examples for middlwares in containers
*/

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func main() {
	system := dcmd.NewStandardSystem("[")

	heyCmd := &StaticCmd{
		Response:    "Hey there buddy",
		Description: "Greets you",
	}

	byeCmd := &StaticCmd{
		Response:    "Bye friendo!",
		Description: "Parting words",
	}

	system.Root.AddCommand(heyCmd, dcmd.NewTrigger("Hello", "Hey"))
	system.Root.AddCommand(byeCmd, dcmd.NewTrigger("Bye", "Bai"))

	container, _ := system.Root.Sub("container", "c")
	container.Description = "Some extra seperated commands"

	container.AddCommand(heyCmd, dcmd.NewTrigger("Hello", "Hey"))
	container.AddCommand(byeCmd, dcmd.NewTrigger("Bye", "Bai"))

	tracker := &CommandsStatTracker{
		CommandUsages: make(map[string]int),
	}

	system.Root.AddMidlewares(tracker.MiddleWare)
	system.Root.AddCommand(tracker, dcmd.NewTrigger("stats"))
	system.Root.AddCommand(dcmd.NewStdHelpCommand(), dcmd.NewTrigger("help", "h"))
	system.Root.BuildMiddlewareChains(nil)

	session, err := discordgo.New(os.Getenv("DG_TOKEN"))
	if err != nil {
		log.Fatal("Failed setting up session:", err)
	}

	session.AddHandler(system.HandleMessageCreate)

	err = session.Open()
	if err != nil {
		log.Fatal("Failed opening gateway connection:", err)
	}
	log.Println("Running, Ctrl-c to stop.")
	select {}
}

// Same commands as used in the simple example
type StaticCmd struct {
	Response    string
	Description string
}

// Compilie time assertions, will not compiled unless StaticCmd implements these interfaces
var _ dcmd.Cmd = (*StaticCmd)(nil)
var _ dcmd.CmdWithDescriptions = (*StaticCmd)(nil)

// Descriptions should return a short description (used in the overall help overiview) and one long descriptions for targetted help
func (s *StaticCmd) Descriptions(d *dcmd.Data) (string, string) { return s.Description, "" }

func (e *StaticCmd) Run(data *dcmd.Data) (interface{}, error) {
	return e.Response, nil
}

// Using this middleware, command usages in the container (and all sub containers) will be counted

type CommandsStatTracker struct {
	CommandUsages     map[string]int
	CommandUsagesLock sync.RWMutex
}

func (c *CommandsStatTracker) MiddleWare(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(d *dcmd.Data) (interface{}, error) {
		// Using the container chain to generate a unique name for this command
		// The container chain is just a slice of all the containers the command is in, the first will always be the root
		name := ""
		for _, c := range d.ContainerChain {
			if len(c.Names) < 1 || c.Names[0] == "" {
				continue
			}
			name += c.Names[0] + " "
		}

		// Finally append the actual command name
		name += d.Cmd.Trigger.Names[0]

		c.CommandUsagesLock.Lock()
		c.CommandUsages[name]++
		c.CommandUsagesLock.Unlock()

		return inner(d)
	}
}

func (c *CommandsStatTracker) Descriptions(d *dcmd.Data) (string, string) {
	return "Shows command usage stats", ""
}

// Sort and dump the stats
func (c *CommandsStatTracker) Run(d *dcmd.Data) (interface{}, error) {
	c.CommandUsagesLock.RLock()
	defer c.CommandUsagesLock.RUnlock()

	out := "```\n"

	for cmdName, usages := range c.CommandUsages {
		out += fmt.Sprintf("%15s: %d\n", cmdName, usages)
	}

	out += "```"
	return out, nil
}
