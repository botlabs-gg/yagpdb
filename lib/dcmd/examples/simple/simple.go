package main

/*
This example provides 2 basic commands with static responses.
The commands `[hey/hello` and `[bye/bai`
*/

import (
	"log"
	"os"

	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func main() {
	system := dcmd.NewStandardSystem("[")
	system.Root.AddCommand(&StaticCmd{
		Response:    "Hey there buddy",
		Description: "Greets you",
	}, dcmd.NewTrigger("Hello", "Hey"))

	system.Root.AddCommand(&StaticCmd{
		Response:    "Bye friendo!",
		Description: "Parting words",
	}, dcmd.NewTrigger("Bye", "Bai"))

	session, err := discordgo.New(os.Getenv("DISCORD_TOKEN"))
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
