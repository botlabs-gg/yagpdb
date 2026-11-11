package cardsagainstdiscord

import (
	"strings"
	"sync"
	"time"

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

type CreateGameOptions struct {
	GuildID   int64
	ChannelID int64
	UserID    int64
	Username  string
	VoteMode  bool

	// Built in packs or names of the custom packs, * for all of them
	Packs       []string
	CustomPacks []*CardPack

	// Zero uses the defaults
	PlayerLimit      int
	WinLimit         int
	PickResponseTime time.Duration
	PickWinnerTime   time.Duration
}

func (gm *GameManager) CreateGame(opts CreateGameOptions) (*Game, error) {
	resolver := &Game{CustomPacks: opts.CustomPacks}
	packs := opts.Packs

	allPacks := false
	allResponseOnly := true
	allPromptOnly := true
	for _, v := range packs {
		if v == "*" {
			allPacks = true
			allResponseOnly = false
			allPromptOnly = false
			break
		}

		p := resolver.resolvePack(v)
		if p == nil {
			validPacks := make([]string, 0, len(Packs)+len(opts.CustomPacks))
			for k := range Packs {
				validPacks = append(validPacks, k)
			}
			for _, cp := range opts.CustomPacks {
				validPacks = append(validPacks, cp.Name)
			}
			return nil, &ErrUnknownPack{
				PassedPack:  v,
				Suggestions: jarowinkler.Select(validPacks, v, jarowinkler.WithLimit(3)),
			}
		}
		if len(p.Prompts) > 0 {
			allResponseOnly = false
		}
		if len(p.Responses) > 0 {
			allPromptOnly = false
		}
	}

	if len(packs) < 1 && !allPacks {
		return nil, ErrNoPacks
	}
	if allResponseOnly {
		return nil, ErrAllPacksResponseOnly
	}
	if allPromptOnly {
		return nil, ErrAllPacksPromptOnly
	}

	if allPacks {
		packs = make([]string, 0, len(Packs)+len(opts.CustomPacks))
		for k := range Packs {
			packs = append(packs, k)
		}
		for _, cp := range opts.CustomPacks {
			packs = append(packs, cp.Name)
		}
	}

	playerLimit, winLimit := opts.PlayerLimit, opts.WinLimit
	if playerLimit <= 0 {
		playerLimit = 10
	}
	// every response is labeled with one of the selection emojis
	playerLimit = min(playerLimit, MaxPlayers)
	if winLimit <= 0 {
		winLimit = 10
	}

	gm.Lock()
	defer gm.Unlock()

	if _, ok := gm.ActiveGames[opts.ChannelID]; ok {
		return nil, ErrGameAlreadyInChannel
	}

	if _, ok := gm.ActiveGames[opts.UserID]; ok {
		return nil, ErrPlayerAlreadyInGame
	}

	game := &Game{
		MasterChannel:    opts.ChannelID,
		Manager:          gm,
		GuildID:          opts.GuildID,
		Packs:            packs,
		CustomPacks:      opts.CustomPacks,
		GameMaster:       opts.UserID,
		VoteMode:         opts.VoteMode,
		PlayerLimit:      playerLimit,
		WinLimit:         winLimit,
		PickResponseTime: opts.PickResponseTime,
		PickWinnerTime:   opts.PickWinnerTime,
		Session:          gm.SessionProvider.SessionForGuild(opts.GuildID),
	}

	err := game.Created()
	if err == nil {
		game.AddPlayer(opts.UserID, opts.Username)

		gm.ActiveGames[opts.ChannelID] = game
		gm.ActiveGames[opts.UserID] = game
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
		}
	}

	gm.NumActiveGames--

	return nil
}

func (gm *GameManager) HandleInteractionCreate(ic *discordgo.InteractionCreate) {
	if ic.Type != discordgo.InteractionMessageComponent && ic.Type != discordgo.InteractionModalSubmit {
		return
	}

	if ic.GuildID == 0 {
		return
	}

	// only the game's own components and modals, others in the channel belong to other features
	var customID string
	if ic.Type == discordgo.InteractionModalSubmit {
		customID = ic.ModalSubmitData().CustomID
	} else {
		customID = ic.MessageComponentData().CustomID
	}
	if !strings.HasPrefix(customID, "cah_") {
		return
	}

	gm.RLock()
	game, ok := gm.ActiveGames[ic.ChannelID]
	gm.RUnlock()

	if ok {
		game.HandleInteractionAdd(ic)
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
