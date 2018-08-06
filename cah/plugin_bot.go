package cah

import (
	"fmt"
	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
)

func RegisterPlugin() {
	p := &Plugin{}
	p.Manager = cardsagainstdiscord.NewGameManager(p)
	common.RegisterPluginL(p)

}

type Plugin struct {
	common.BasePlugin
	Manager *cardsagainstdiscord.GameManager
}

func (p *Plugin) Name() string {
	return "CAH"
}

func (p *Plugin) SessionForGuild(guildID int64) *discordgo.Session {
	return common.BotSession
}

var (
	_ bot.BotInitHandler       = (*Plugin)(nil)
	_ commands.CommandProvider = (*Plugin)(nil)
)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(func(evt *eventsystem.EventData) {
		switch t := evt.EvtInterface.(type) {
		case *discordgo.MessageCreate:
			go p.Manager.HandleMessageCreate(t)
		case *discordgo.MessageReactionAdd:
			go p.Manager.HandleReactionAdd(t)
		}
	}, eventsystem.EventMessageReactionAdd, eventsystem.EventMessageCreate)
}

func (p *Plugin) Status() (string, string) {
	p.Manager.RLock()
	var countedGames []int64

	games := 0
	totalPlayers := 0
	activePlayers := 0

	for _, v := range p.Manager.ActiveGames {
		if common.ContainsInt64Slice(countedGames, v.MasterChannel) {
			continue
		}
		p.Manager.RUnlock()
		v.RLock()

		games++
		totalPlayers += len(v.Players)
		for _, p := range v.Players {
			if p.InGame {
				activePlayers++
			}
		}

		v.RUnlock()
		p.Manager.RLock()

		countedGames = append(countedGames, v.MasterChannel)
	}

	return "Games/ Tot. players / Act. players", fmt.Sprintf("%d / %d / %d", games, totalPlayers, activePlayers)
}
