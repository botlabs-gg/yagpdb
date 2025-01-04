package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate/inmemorytracker"
)

var cahManager *cardsagainstdiscord.GameManager

func panicErr(err error, msg string) {
	if err != nil {
		panic(msg + ": " + err.Error())
	}
}

func main() {
	session, err := discordgo.New(os.Getenv("DG_TOKEN"))
	session.Intents = []discordgo.GatewayIntent{
		discordgo.GatewayIntentGuilds,
		discordgo.GatewayIntentGuildExpressions,
		discordgo.GatewayIntentGuildMessages,
		discordgo.GatewayIntentGuildMessageReactions,
	}
	panicErr(err, "Failed initializing discordgo")

	cahManager = cardsagainstdiscord.NewGameManager(&cardsagainstdiscord.StaticSessionProvider{
		Session: session,
	})

	state := inmemorytracker.NewInMemoryTracker(inmemorytracker.TrackerConfig{}, 1)
	session.StateEnabled = false

	cmdSys := dcmd.NewStandardSystem("!cah")
	cmdSys.State = state
	cmdSys.Root.AddCommand(CreateGameCommand, dcmd.NewTrigger("create", "c").SetEnableInGuildChannels(true))
	cmdSys.Root.AddCommand(StopCommand, dcmd.NewTrigger("stop", "end", "s").SetEnableInGuildChannels(true))
	cmdSys.Root.AddCommand(KickCommand, dcmd.NewTrigger("kick").SetEnableInGuildChannels(true))
	cmdSys.Root.AddCommand(PacksCommand, dcmd.NewTrigger("packs").SetEnableInGuildChannels(true))

	session.AddHandler(state.HandleEvent)
	session.AddHandler(cmdSys.HandleMessageCreate)
	session.AddHandler(func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		go cahManager.HandleInteractionCreate(ic)
	})

	err = session.Open()
	panicErr(err, "Failed opening gateway connection")
	log.Println("Running...")

	// We import http/pprof above to be ale to inspect shizz and do profiling
	go http.ListenAndServe(":7447", nil)
	select {}
}

var CreateGameCommand = &dcmd.SimpleCmd{
	ShortDesc: "Creates a cards against humanity game in this channel",
	CmdArgDefs: []*dcmd.ArgDef{
		{Name: "packs", Type: dcmd.String, Default: "main", Help: "Packs seperated by space, or * to include all of them"},
	},
	CmdSwitches: []*dcmd.ArgDef{
		{Name: "v", Help: "Vote mode, no cardczar"},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		voteMode := data.Switch("v").Bool()
		pStr := data.Args[0].Str()
		packs := strings.Fields(pStr)

		_, err := cahManager.CreateGame(data.GuildData.GS.ID, data.GuildData.CS.ID, data.Author.ID, data.Author.Username, voteMode, packs...)
		if err == nil {
			log.Println("Created a new game in ", data.GuildData.CS.ID)
			return "", nil
		}

		if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
			return cahErr, nil
		}

		return "Something went wrong", err

	},
}

var StopCommand = &dcmd.SimpleCmd{
	ShortDesc: "Stops a cards against humanity game in this channel",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		err := cahManager.TryAdminRemoveGame(data.Author.ID)
		if err != nil {
			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "Something went wrong", err
		}

		return "Stopped the game", nil
	},
}

var KickCommand = &dcmd.SimpleCmd{
	ShortDesc:       "Kicks a player from the card against humanity game in this channel, only the game master can do this",
	RequiredArgDefs: 1,
	CmdArgDefs: []*dcmd.ArgDef{
		{Name: "user", Type: dcmd.UserID},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		userID := data.Args[0].Int64()

		err := cahManager.AdminKickUser(data.Author.ID, userID)
		if err != nil {
			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "Something went wrong", err
		}

		return "User removed", nil
	},
}

var PacksCommand = &dcmd.SimpleCmd{
	ShortDesc: "Lists available packs",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		resp := "Available packs: \n\n"
		for _, v := range cardsagainstdiscord.Packs {
			resp += "`" + v.Name + "` - " + v.Description + "\n"
		}

		return resp, nil
	},
}
