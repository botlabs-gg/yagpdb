package cah

import (
	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"strings"
)

func (p *Plugin) AddCommands() {

	cmdCreate := &commands.YAGCommand{
		Name:        "create",
		CmdCategory: commands.CategoryFun,
		Aliases:     []string{"c"},
		Description: "Creates a cards against humanity game in this channel, add packs after commands, or * for all packs. (-v for vote mode without a card czar)",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "packs", Type: dcmd.String, Default: "main", Help: "Packs seperated by space, or * for all of them"},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Switch: "v", Name: "Vote mode - players vote instead of having a cardczar"},
		},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			voteMode := data.Switch("v").Bool()
			pStr := data.Args[0].Str()
			packs := strings.Fields(pStr)

			_, err := p.Manager.CreateGame(data.GS.ID, data.CS.ID, data.Msg.Author.ID, data.Msg.Author.Username, voteMode, packs...)
			if err == nil {
				p.Logger().Info("Created a new game in ", data.CS.ID, ":", data.GS.ID)
				return "", nil
			}

			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "", err
		},
	}

	cmdEnd := &commands.YAGCommand{
		Name:        "end",
		CmdCategory: commands.CategoryFun,
		Description: "Ends a cards against humanity game thats ongoing in this channel.",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			isAdmin, err := bot.AdminOrPerm(0, data.Msg.Author.ID, data.CS.ID)
			if err == nil && isAdmin {
				err = p.Manager.RemoveGame(data.CS.ID)
			} else {
				err = p.Manager.TryAdminRemoveGame(data.Msg.Author.ID)
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
		Name:         "kick",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "user", Type: dcmd.UserID},
		},
		Description: "Kicks a player from the ongoing cards against humanity game in this channel.",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			userID := data.Args[0].Int64()
			err := p.Manager.AdminKickUser(data.Msg.Author.ID, userID)
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
		Name:         "packs",
		CmdCategory:  commands.CategoryFun,
		RequiredArgs: 0,
		Description:  "Lists available packs",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			resp := "Available packs: \n\n"
			for _, v := range cardsagainstdiscord.Packs {
				resp += "`" + v.Name + "` - " + v.Description + "\n"
			}

			return resp, nil
		},
	}

	container := commands.CommandSystem.Root.Sub("cah")
	container.NotFound = func(data *dcmd.Data) (interface{}, error) {
		resp := dcmd.GenerateHelp(data, container, &dcmd.StdHelpFormatter{})
		if len(resp) > 0 {
			return resp[0], nil
		}

		return "Unknown cah command", nil
	}

	container.AddCommand(cmdCreate, cmdCreate.GetTrigger())
	container.AddCommand(cmdEnd, cmdEnd.GetTrigger())
	container.AddCommand(cmdKick, cmdKick.GetTrigger())
	container.AddCommand(cmdPacks, cmdPacks.GetTrigger())
}
