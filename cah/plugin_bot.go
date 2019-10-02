package cah

import (
	"fmt"

	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dshardorchestrator"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
)

const ShardMigrationEvtGame = 110

func init() {
	dshardorchestrator.RegisterUserEvent("CAHGame", ShardMigrationEvtGame, cardsagainstdiscord.Game{})
}

func RegisterPlugin() {
	p := &Plugin{}
	p.Manager = cardsagainstdiscord.NewGameManager(p)
	common.RegisterPlugin(p)

}

type Plugin struct {
	Manager *cardsagainstdiscord.GameManager
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "CAH",
		SysName:  "cah",
		Category: common.PluginCategoryMisc,
	}
}

func (p *Plugin) SessionForGuild(guildID int64) *discordgo.Session {
	return common.BotSession
}

var (
	_ bot.BotInitHandler         = (*Plugin)(nil)
	_ bot.ShardMigrationReceiver = (*Plugin)(nil)
	_ bot.ShardMigrationSender   = (*Plugin)(nil)
	_ commands.CommandProvider   = (*Plugin)(nil)
)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, func(evt *eventsystem.EventData) {
		switch t := evt.EvtInterface.(type) {
		case *discordgo.MessageCreate:
			if t.GuildID == 0 {
				return
			}

			go p.Manager.HandleMessageCreate(t)
		case *discordgo.MessageReactionAdd:
			if t.GuildID == 0 {
				return
			}

			go p.Manager.HandleReactionAdd(t)
		}
	}, eventsystem.EventMessageReactionAdd, eventsystem.EventMessageCreate)

	pubsub.AddHandler("dm_reaction", func(evt *pubsub.Event) {
		dataCast := evt.Data.(*discordgo.MessageReactionAdd)
		go p.Manager.HandleReactionAdd(dataCast)
	}, discordgo.MessageReactionAdd{})

	pubsub.AddHandler("dm_message", func(evt *pubsub.Event) {
		dataCast := evt.Data.(*discordgo.MessageCreate)
		go p.Manager.HandleMessageCreate(dataCast)
	}, discordgo.MessageCreate{})
}

func (p *Plugin) Status() (string, string) {
	p.Manager.RLock()
	defer p.Manager.RUnlock()

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

	return "Games / Tot. players / Act. players", fmt.Sprintf("%d / %d / %d", games, totalPlayers, activePlayers)
}

func (p *Plugin) ShardMigrationSend(shard int) int {

	p.Manager.Lock()
	games := make([]*cardsagainstdiscord.Game, 0)
OUTER:
	for k, v := range p.Manager.ActiveGames {
		if bot.GuildShardID(v.GuildID) != shard {
			continue
		}

		// were migrating it
		delete(p.Manager.ActiveGames, k)
		for _, alreadyAdded := range games {
			if alreadyAdded == v {
				continue OUTER
			}
		}

		games = append(games, v)
	}
	p.Manager.Unlock()

	for _, v := range games {
		v.Stop()

		v.Lock()
		common.BotSession.ChannelMessageSend(v.MasterChannel, "**Cards Against Humanity:** Bot undergoing upgrade: Your game will resume in around 10 seconds.")
		bot.NodeConn.Send(ShardMigrationEvtGame, v, false)
		v.Unlock()
	}

	return len(games)
}

func (p *Plugin) ShardMigrationReceive(evt dshardorchestrator.EventType, data interface{}) {
	if evt != ShardMigrationEvtGame {
		return
	}

	game := data.(*cardsagainstdiscord.Game)
	p.Manager.LoadGameFromSerializedState(game)
}
