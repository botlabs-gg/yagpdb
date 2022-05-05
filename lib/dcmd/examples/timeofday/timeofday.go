package main

/*
This example provides a single command "time" that responds with the current time of day
*/

import (
	"log"
	"os"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func main() {
	// Create a new command system
	system := dcmd.NewStandardSystem("[")

	// Add the time of day command to the root container of the system
	system.Root.AddCommand(&CmdTimeOfDay{Format: time.RFC822}, dcmd.NewTrigger("Time", "t"))

	// Create the discordgo session
	session, err := discordgo.New(os.Getenv("DG_TOKEN"))
	if err != nil {
		log.Fatal("Failed setting up session:", err)
	}

	// Add the command system handler to discordgo
	session.AddHandler(system.HandleMessageCreate)

	err = session.Open()
	if err != nil {
		log.Fatal("Failed opening gateway connection:", err)
	}
	log.Println("Running, Ctrl-c to stop.")
	select {}
}

type CmdTimeOfDay struct {
	Format string
}

// Descriptions should return a short description (used in the overall help overiview) and one long descriptions for targetted help
func (t *CmdTimeOfDay) Descriptions(d *dcmd.Data) (string, string) {
	return "Responds with the current time in utc", ""
}

// Run implements the dcmd.Cmd interface and gets called when the command is invoked
func (t *CmdTimeOfDay) Run(data *dcmd.Data) (interface{}, error) {
	return time.Now().UTC().Format(t.Format), nil
}

// Compilie time assertions, will not compiled unless StaticCmd implements these interfaces
var _ dcmd.Cmd = (*CmdTimeOfDay)(nil)
var _ dcmd.CmdWithDescriptions = (*CmdTimeOfDay)(nil)
