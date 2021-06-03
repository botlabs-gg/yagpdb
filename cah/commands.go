package cah

import (
	"strings"

	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/dcmd/v3"
	"github.com/jonas747/dstate/v3"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/sirupsen/logrus"
)

func (p *Plugin) AddCommands() {

	cmdCreate := &commands.YAGCommand{
		Name:        "Create",
		CmdCategory: commands.CategoryFun,
		Aliases:     []string{"c"},
		Description: "Creates a Cards Against Humanity game in this channel, add packs after commands, or * for all packs. (-v for vote mode without a card czar).",
		Arguments: []*dcmd.ArgDef{
			{Name: "packs", Type: dcmd.String, Default: "main", Help: "Packs separated by space, or * for all of them."},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "v", Help: "Vote mode - players vote instead of having a card czar."},
		},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			voteMode := data.Switch("v").Bool()
			pStr := data.Args[0].Str()
			packs := strings.Fields(pStr)

			_, err := p.Manager.CreateGame(data.GuildData.GS.ID, data.GuildData.CS.ID, data.Author.ID, data.Author.Username, voteMode, packs...)
			if err == nil {
				logrus.Info("[cah] Created a new game in ", data.GuildData.CS.ID, ":", data.GuildData.GS.ID)
				return "", nil
			}

			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "", err
		},
	}

	cmdEnd := &commands.YAGCommand{
		Name:        "End",
		CmdCategory: commands.CategoryFun,
		Description: "Ends a Cards Against Humanity game that is ongoing in this channel.",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			isAdmin, err := bot.AdminOrPermMS(data.GuildData.GS.ID, data.ChannelID, data.GuildData.MS, 0)
			if err == nil && isAdmin {
				err = p.Manager.RemoveGame(data.ChannelID)
			} else {
				err = p.Manager.TryAdminRemoveGame(data.Author.ID)
			}

			if err != nil {
				if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
					return cahErr, nil
				}

				return "", err
			}

			return "Stopped the game", nil
		},
	}

	cmdKick := &commands.YAGCommand{
		Name:         "Kick",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "user", Type: dcmd.UserID},
		},
		Description: "Kicks a player from the ongoing Cards Against Humanity game in this channel.",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			userID := data.Args[0].Int64()
			err := p.Manager.AdminKickUser(data.Author.ID, userID)
			if err != nil {
				if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
					return cahErr, nil
				}

				return "", err
			}

			return "User removed", nil
		},
	}

	cmdPacks := &commands.YAGCommand{
		Name:         "Packs",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 0,
		Description:  "Lists all available packs.",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			resp := "Available packs: \n\n"
			for _, v := range cardsagainstdiscord.Packs {
				resp += "`" + v.Name + "` - " + v.Description + "\n"
			}

			return resp, nil
		},
	}

	container := commands.CommandSystem.Root.Sub("cah")
	container.NotFound = commands.CommonContainerNotFoundHandler(container, "")
	container.Description = "Playt cards against humanity!"

	container.AddCommand(cmdCreate, cmdCreate.GetTrigger())
	container.AddCommand(cmdEnd, cmdEnd.GetTrigger())
	container.AddCommand(cmdKick, cmdKick.GetTrigger())
	container.AddCommand(cmdPacks, cmdPacks.GetTrigger())
	commands.RegisterSlashCommandsContainer(container, true, func(gs *dstate.GuildSet) ([]int64, error) {
		return nil, nil
	})
}
