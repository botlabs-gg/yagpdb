package cardsagainstdiscord

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/jarowinkler"
)

type GameManager struct {
	sync.RWMutex
	SessionProvider SessionProvider
	ActiveGames     map[int64]*Game
	NumActiveGames  int
}

func NewGameManager(sessionProvider SessionProvider) *GameManager {
	return &GameManager{
		ActiveGames:     make(map[int64]*Game),
		SessionProvider: sessionProvider,
	}
}

func (gm *GameManager) CreateGame(guildID int64, channelID int64, userID int64, username string, voteMode bool, packs ...string) (*Game, error) {
	allPacks := false
	allResponseOnly := true
	for _, v := range packs {
		if v == "*" {
			allPacks = true
			allResponseOnly = false
			break
		}

		p, ok := Packs[v]
		if !ok {
			validPacks := make([]string, 0, len(Packs))
			for k := range Packs {
				validPacks = append(validPacks, k)
			}
			return nil, &ErrUnknownPack{
				PassedPack:  v,
				Suggestions: jarowinkler.Select(validPacks, v, jarowinkler.WithLimit(3)),
			}
		}
		if len(p.Prompts) > 0 {
			allResponseOnly = false
		}
	}

	if len(packs) < 1 && !allPacks {
		return nil, ErrNoPacks
	}
	if allResponseOnly {
		return nil, ErrAllPacksResponseOnly
	}

	if allPacks {
		packs = make([]string, 0, len(Packs))
		for k := range Packs {
			packs = append(packs, k)
		}
	}

	gm.Lock()
	defer gm.Unlock()

	if _, ok := gm.ActiveGames[channelID]; ok {
		return nil, ErrGameAlreadyInChannel
	}

	if _, ok := gm.ActiveGames[userID]; ok {
		return nil, ErrPlayerAlreadyInGame
	}

	game := &Game{
		MasterChannel: channelID,
		Manager:       gm,
		GuildID:       guildID,
		Packs:         packs,
		GameMaster:    userID,
		VoteMode:      voteMode,
		PlayerLimit:   10,
		WinLimit:      10,
		Session:       gm.SessionProvider.SessionForGuild(guildID),
	}

	err := game.Created()
	if err == nil {
		game.AddPlayer(userID, username)

		gm.ActiveGames[channelID] = game
		gm.ActiveGames[userID] = game
		gm.NumActiveGames++
	}

	return game, err
}

func (gm *GameManager) FindGameFromChannelOrUser(id int64) *Game {
	gm.RLock()
	defer gm.RUnlock()

	if g, ok := gm.ActiveGames[id]; ok {
		return g
	}

	return nil
}

func (gm *GameManager) PlayerTryJoinGame(gameID, playerID int64, username string) error {
	gm.Lock()
	defer gm.Unlock()

	if _, ok := gm.ActiveGames[playerID]; ok {
		return ErrPlayerAlreadyInGame
	}

	if g, ok := gm.ActiveGames[gameID]; ok {
		if g.AddPlayer(playerID, username) {
			gm.ActiveGames[playerID] = g
			return nil
		}

		return ErrGameFull
	}

	return ErrGameNotFound
}

func (gm *GameManager) PlayerTryLeaveGame(playerID int64) error {
	gm.Lock()
	defer gm.Unlock()

	if g, ok := gm.ActiveGames[playerID]; ok {
		delete(gm.ActiveGames, playerID)
		g.RemovePlayer(playerID)
		return nil
	}

	return ErrGameNotFound
}

func (gm *GameManager) AdminKickUser(admin, playerID int64) error {
	gm.Lock()
	defer gm.Unlock()

	g, ok := gm.ActiveGames[admin]
	if !ok {
		return ErrGameNotFound
	}

	g.RLock()
	if g.GameMaster != admin {
		g.RUnlock()
		return ErrNotGM
	}
	g.RUnlock()

	if g.RemovePlayer(playerID) {
		delete(gm.ActiveGames, playerID)
	} else {
		return ErrPlayerNotInGame
	}

	return nil
}

func (p *Player) removeLastMenuReactions(session *discordgo.Session) {
	if p.LastReactionMenu != 0 {
		session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    p.Channel,
			ID:         p.LastReactionMenu,
			Components: []discordgo.TopLevelComponent{},
		})
	}
}

func (gm *GameManager) RemoveGame(gameID int64) error {
	gm.Lock()
	defer gm.Unlock()

	g, ok := gm.ActiveGames[gameID]
	if !ok {
		return ErrGameNotFound
	}

	g.Stop()

	// Remove all references to the game
	g.RLock()
	defer g.RUnlock()

	delete(gm.ActiveGames, g.MasterChannel)
	delete(gm.ActiveGames, g.GameMaster)
	g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
	for _, v := range g.Players {
		if v.InGame {
			delete(gm.ActiveGames, v.ID)
			v.removeLastMenuReactions(g.Session)
		}
	}

	gm.NumActiveGames--

	return nil
}

func (gm *GameManager) TryAdminRemoveGame(admin int64) error {
	gm.Lock()
	defer gm.Unlock()

	g, ok := gm.ActiveGames[admin]
	if !ok {
		return ErrGameNotFound
	}

	g.Lock()
	defer g.Unlock()

	if g.GameMaster != admin {
		return ErrNotGM
	}

	if g.stopped {
		return ErrStoppedAlready
	}

	close(g.stopch)
	g.stopped = true

	// Remove all references to the game
	delete(gm.ActiveGames, g.MasterChannel)
	delete(gm.ActiveGames, g.GameMaster)
	g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
	for _, v := range g.Players {
		if v.InGame {
			delete(gm.ActiveGames, v.ID)
			v.removeLastMenuReactions(g.Session)
		}
	}

	gm.NumActiveGames--

	return nil
}

func (gm *GameManager) HandleInteractionCreate(ic *discordgo.InteractionCreate) {
	if ic.Type != discordgo.InteractionMessageComponent {
		return
	}

	if ic.GuildID == 0 {
		//DM interactions are handled via pubsub
		return
	}

	gm.HandleCahInteraction(ic)
}

func (gm *GameManager) HandleCahInteraction(ic *discordgo.InteractionCreate) {
	cid := ic.ChannelID
	gm.RLock()

	if game, ok := gm.ActiveGames[cid]; ok {
		gm.RUnlock()
		game.HandleInteractionAdd(ic)
	} else if ic.User != nil {
		gm.RUnlock()
		if ic.Type == discordgo.InteractionModalSubmit && ic.ModalSubmitData().CustomID == CahBlankCardModal {
			if game, ok := gm.ActiveGames[ic.User.ID]; ok {
				game.HandleMessageCreate(ic)
			}
			return
		}
		if game, ok := gm.ActiveGames[ic.User.ID]; ok {
			game.HandleInteractionAdd(ic)
		}
	} else {
		gm.RUnlock()
	}
}

func (gm *GameManager) LoadGameFromSerializedState(game *Game) {
	game.Session = gm.SessionProvider.SessionForGuild(game.GuildID)
	game.Manager = gm
	game.stopch = make(chan bool)

	gm.Lock()
	for _, v := range game.Players {
		if v.InGame {
			gm.ActiveGames[v.ID] = game
		}
	}

	gm.ActiveGames[game.MasterChannel] = game
	gm.NumActiveGames++
	gm.Unlock()

	game.loadFromSerializedState()
}
