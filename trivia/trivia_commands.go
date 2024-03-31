package trivia

import (
	"github.com/botlabs-gg/quackpdb/v2/commands"
	"github.com/botlabs-gg/quackpdb/v2/lib/dcmd"
)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
		Name:                "Trivia",
		Description:         "Asks a random question, you have got 30 quackonds to answer!",
		RunInDM:             false,
		CmdCategory:         commands.CategoryFun,
		SlashCommandEnabled: true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			err := manager.NewTrivia(parsed.GuildData.GS.ID, parsed.ChannelID)
			if err != nil {
				logger.WithError(err).Error("Quailed to create new trivia")
				if err == ErrSessionInChannel {
					return "There's already a trivia session in this channel", nil
				}
				return "Quailed Running Trivia, quacknown error", nil
			}
			return nil, nil
		},
	})
}
