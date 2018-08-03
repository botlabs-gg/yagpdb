package cah

import (
	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/commands"
)

func (p *Plugin) AddCommands() {

	cmdCreate := &commands.YAGCommand{
		Name:        "create",
		CmdCategory: commands.CategoryFun,
		Aliases:     []string{"c"},
		Description: "Creates a cards against humanity game in this channel",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			_, err := p.Manager.CreateGame(data.GS.ID, data.CS.ID, data.Msg.Author.ID, data.Msg.Author.Username, []string{"main"})
			if err == nil {
				p.Logger().Info("Created a new game in ", data.CS.ID, ":", data.GS.ID)
				return "", nil
			}

			if err == cardsagainstdiscord.ErrGameAlreadyInChanenl {
				return "Already a active game in this channel", nil
			} else if err == cardsagainstdiscord.ErrPlayerAlreadyInGame {
				return "You're alrady playing in another game", nil
			} else {
				return "Something went wrong", err
			}

			return "", nil
		},
	}

	cmdEnd := &commands.YAGCommand{
		Name:        "end",
		CmdCategory: commands.CategoryFun,
		Description: "Ends a cards against humanity game thats ongoing in this channel",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			game := p.Manager.FindGameFromChannelOrUser(data.Msg.Author.ID)
			if game == nil {
				return "Couln't find any game you're part of", nil
			}

			if game.GameMaster != data.Msg.Author.ID {
				return "You're not the game master of this game", nil
			}

			err := p.Manager.RemoveGame(data.CS.ID)
			if err != nil {
				if err == cardsagainstdiscord.ErrGameNotFound {
					return "Couldn't find any game you're part of", nil
				} else {
					return "Something bad happened", err
				}
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
		Description: "Kicks a player from the ongoing cards against humanity game in this channel",
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			game := p.Manager.FindGameFromChannelOrUser(data.Msg.Author.ID)
			if game == nil {
				return "Couln't find any game you're part of", nil
			}

			if game.GameMaster != data.Msg.Author.ID {
				return "You're not the game master of this game", nil
			}

			userID := data.Args[0].Int64()

			err := p.Manager.PlayerTryLeaveGame(userID)
			if err != nil {
				if err == cardsagainstdiscord.ErrGameNotFound {
					return "This user is not part of any game anymore", nil
				} else {
					return "Something bad happened", err
				}
			}

			return "User removed", nil
		},
	}

	container := commands.CommandSystem.Root.Sub("cah")
	container.AddCommand(cmdCreate, cmdCreate.GetTrigger())
	container.AddCommand(cmdEnd, cmdEnd.GetTrigger())
	container.AddCommand(cmdKick, cmdKick.GetTrigger())
}
