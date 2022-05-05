package main

/*
This example provides 2 basic commands with static responses.
*/

import (
	"log"
	"os"

	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func main() {
	modCat := &dcmd.Category{
		Name:        "Moderation",
		Description: "Moderation commands",
		HelpEmoji:   "👮",
		EmbedColor:  0xdb0606,
	}

	system := dcmd.NewStandardSystem("[")
	system.Root.AddCommand(dcmd.NewStdHelpCommand(), dcmd.NewTrigger("Help", "h"))
	system.Root.AddCommand(&StaticCmd{
		Desc:     "Shows bot status",
		LongDesc: "Shows bot status such as uptime, and how many resources the bot uses",
	}, dcmd.NewTrigger("Status", "st"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Shows general bot information",
	}, dcmd.NewTrigger("Info", "i"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Ask the bot a yes/no question",
	}, dcmd.NewTrigger("8ball", "ball", "8"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Pokes a user on your server",
	}, dcmd.NewTrigger("Poke"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Warns a user",
		Cat:  modCat,
	}, dcmd.NewTrigger("Warn"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Kicks a user",
		Cat:  modCat,
	}, dcmd.NewTrigger("Kick"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Bans a user",
		Cat:  modCat,
	}, dcmd.NewTrigger("Ban"))
	system.Root.AddCommand(&StaticCmd{
		Desc: "Mutes a user",
		Cat:  modCat,
	}, dcmd.NewTrigger("Mute"))

	musicContainer, _ := system.Root.Sub("music", "m")
	musicContainer.HelpOwnEmbed = true
	musicContainer.HelpColor = 0xd60eab
	musicContainer.HelpTitleEmoji = "🎶"
	musicContainer.AddCommand(&StaticCmd{
		Desc:     "Joins your current voice channel",
		LongDesc: "Makes the bot join your current voice channel, can also be used to move it.",
	}, dcmd.NewTrigger("join", "j"))

	musicContainer.AddCommand(&StaticCmd{
		Desc: "Queues up or starts playing a song, either by url or by searching what you wrote",
		LongDesc: "Queues up or starts playing a song, either by url or by searching what you wrote\nExamples:\n" +
			"`play c2c down the road` - will search for the song and play the first search result\n`play https://www.youtube.com/watch?v=k1uUIJPD0Nk` - will play the specific linked video",
	}, dcmd.NewTrigger("Play", "p"))

	musicContainer.AddCommand(&StaticCmd{
		Desc: "Shows the current queue",
	}, dcmd.NewTrigger("Queue", "q"))
	musicContainer.AddCommand(&StaticCmd{
		Desc: "Skips the current video, if you're not a moderator the majority will have to vote in favor",
	}, dcmd.NewTrigger("Skip", "S"))

	musicContainer.AddCommand(&StaticCmd{
		Desc: "Sets the volume, accepts a number between `1-100`",
	}, dcmd.NewTrigger("Volume", "vol", "v"))

	session, err := discordgo.New(os.Getenv("DG_TOKEN"))
	if err != nil {
		log.Fatal("Failed setting up session:", err)
	}

	session.AddHandler(system.HandleMessageCreate)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Ready recevied")
	})
	err = session.Open()
	if err != nil {
		log.Fatal("Failed opening gateway connection:", err)
	}
	log.Println("Running, Ctrl-c to stop.")
	select {}
}

type StaticCmd struct {
	Resp           string
	Desc, LongDesc string
	Cat            *dcmd.Category
}

// Compilie time assertions, will not compiled unless StaticCmd implements these interfaces
var _ dcmd.Cmd = (*StaticCmd)(nil)
var _ dcmd.CmdWithDescriptions = (*StaticCmd)(nil)
var _ dcmd.CmdWithCategory = (*StaticCmd)(nil)

// Descriptions should return a short Desc (used in the overall help overiview) and one long descriptions for targetted help
func (s *StaticCmd) Descriptions(d *dcmd.Data) (string, string) { return s.Desc, "" }

func (e *StaticCmd) Run(data *dcmd.Data) (interface{}, error) {
	if e.Resp == "" {
		return "Mock response", nil
	}
	return e.Resp, nil
}

func (e *StaticCmd) Category() *dcmd.Category {
	return e.Cat
}
