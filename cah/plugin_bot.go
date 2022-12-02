package cah

import (
	"fmt"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/pubsub"
	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator"
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
		ic := evt.EvtInterface.(*discordgo.InteractionCreate)
		if ic.GuildID == 0 {
			return
		}
		go p.Manager.HandleInteractionCreate(ic)
	}, eventsystem.EventInteractionCreate)

	pubsub.AddHandler("dm_interaction", func(evt *pubsub.Event) {
		dataCast := evt.Data.(*discordgo.InteractionCreate)
		if dataCast.Type != discordgo.InteractionMessageComponent && dataCast.Type != discordgo.InteractionModalSubmit {
			return
		}
		go p.Manager.HandleCahInteraction(dataCast)
	}, discordgo.InteractionCreate{})
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
		if bot.GuildShardID(int64(bot.ShardManager.GetNumShards()), v.GuildID) != shard {
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
