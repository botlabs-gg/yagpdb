package trivia

import (
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
		Name:                "Trivia",
		Description:         "Asks a random question, you have got 30 seconds to answer!",
		RunInDM:             false,
		SlashCommandEnabled: true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			err := manager.NewTrivia(parsed.GuildData.GS.ID, parsed.ChannelID)
			if err != nil {
				if err == ErrSessionInChannel {
					return "There's already a trivia session in this channel", nil
				}
				return nil, err
			}
			return nil, nil
		},
	})
}
