package cardsagainstdiscord

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/sirupsen/logrus"
)

type GameState int

/*

Game Loop:      If a player has enough wins to win the game
pregame ------------------<|
   |                       |
PreRoundDelay ------------<|
   |                       |
PickingResponses           |
   |                       |
Picking winner             |
   |>----------------------^

*/

const (
	GameStatePreGame          GameState = 0 // Before the game starts
	GameStatePreRoundDelay    GameState = 1 // Countdown before a round starts
	GameStatePickingResponses GameState = 2 // Players are picking responses for the prompt card
	GameStatePickingWinner    GameState = 3 // Cardzar is picking the winning response
	GameStateEnded            GameState = 4 // Game is over, someone won
)

const (
	PreRoundDelayDuration  = time.Second * 15
	PickResponseDuration   = time.Second * 60
	PickWinnerDuration     = time.Second * 90
	GameExpireAfter        = time.Second * 300
	GameExpireAfterPregame = time.Minute * 30

	BlankCard ResponseCard = "(write your own response)"
)

var (
	CardSelectionEmojis = []string{
		"🇦", // A
		"🇧", // B
		"🇨", // C
		"🇩", // D
		"🇪", // E
		"🇫", // F
		"🇬", // G
		"🇭", // H
		"🇮", // I
		"🇯", // J
		"🇰", // K
	}

	JoinEmoji      = "➕"
	LeaveEmoji     = "➖"
	PlayPauseEmoji = "⏯"

	CahGameJoined    = "cah_game_joined"
	CahGameLeft      = "cah_game_left"
	CahGamePlayPause = "cah_game_play"

	CahCardSelectMenu = "cah_card_select"
	CahBlankCardModal = "cah_blank_card"
	CahTextInput      = "cah_text_input"
)

type Game struct {
	sync.RWMutex `json:"-" msgpack:"-"`
	// Never chaged
	Manager *GameManager       `json:"-" msgpack:"-"`
	Session *discordgo.Session `json:"-" msgpack:"-"`

	// The main channel this game resides in, never changes
	MasterChannel int64
	// The server the game resides in, never changes
	GuildID int64

	// The user that created this game
	GameMaster int64

	// The current cardzar
	CurrentCardCzar int64

	PlayerLimit        int
	WinLimit           int
	VoteMode           bool
	Packs              []string
	availablePrompts   []*PromptCard
	availableResponses []ResponseCard

	Players []*Player

	State        GameState
	StateEntered time.Time

	// The time the most recent action was taken, if we go too long without a user action we expire the game
	LastAction time.Time

	CurrentPropmpt *PromptCard

	LastMenuMessage int64

	Responses []*PickedResonse

	stopped       bool
	tickerRunning bool
	stopch        chan bool
}

type PickedResonse struct {
	Player     *Player
	Selections []ResponseCard
}

func GetCommonCahButtons() []discordgo.TopLevelComponent {
	return []discordgo.TopLevelComponent{
		discordgo.ActionsRow{
			Components: []discordgo.InteractiveComponent{
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: JoinEmoji},
					Style:    discordgo.SuccessButton,
					CustomID: CahGameJoined,
				},
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: LeaveEmoji},
					Style:    discordgo.DangerButton,
					CustomID: CahGameLeft,
				},
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: PlayPauseEmoji},
					Style:    discordgo.PrimaryButton,
					CustomID: CahGamePlayPause,
				},
			},
		},
	}
}

func (g *Game) Created() error {
	g.LastAction = time.Now()

	embed := &discordgo.MessageEmbed{
		Title:       "Game created!",
		Description: fmt.Sprintf("React with %s to join and %s to leave, the game master can start/stop the game with %s", JoinEmoji, LeaveEmoji, PlayPauseEmoji),
	}

	msg, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Components: GetCommonCahButtons(),
		Embeds:     []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		return err
	}

	g.LastMenuMessage = msg.ID

	g.stopch = make(chan bool)

	g.loadPackPrompts()
	g.loadPackResponses()

	go g.runTicker()

	g.tickerRunning = true
	return nil
}

func (g *Game) loadPackResponses() {
	for _, v := range g.Packs {
		pack := Packs[v]
		g.availableResponses = append(g.availableResponses, pack.Responses...)
	}
}
func (g *Game) loadPackPrompts() {
	for _, v := range g.Packs {
		pack := Packs[v]
		g.availablePrompts = append(g.availablePrompts, pack.Prompts...)
	}
}

// AddPlayer attempts to add a player to the game, if it fails (hit the limit for example) then it returns false
func (g *Game) AddPlayer(id int64, username string) bool {
	g.Lock()
	defer g.Unlock()

	return g.addPlayer(id, username)
}

func (g *Game) addPlayer(id int64, username string) bool {
	// 500 is max capacity
	if 500 <= len(g.Players) {
		return false
	}

	numPlaying := 0
	for _, v := range g.Players {
		if v.InGame {
			numPlaying++
		}
	}

	if numPlaying >= g.PlayerLimit {
		return false
	}

	msg := ""

	// Create a userchannel and cache it for use later
	existing := g.findPlayer(id)
	if existing != nil {

		if existing.Banned {
			return false
		}

		existing.InGame = true
		msg = "Came back!"
	} else {

		channel, err := g.Session.UserChannelCreate(id)
		if err != nil {
			return false
		}

		p := &Player{
			ID:       id,
			Username: username,
			Channel:  channel.ID,
			Cards:    g.getRandomPlayerCards(8),
			InGame:   true,
		}

		g.Players = append(g.Players, p)

		msg = "Joined the game!"
	}

	go g.sendAnnouncment(fmt.Sprintf("<@%d> %s! (%d/%d)", id, msg, numPlaying+1, g.PlayerLimit), false)
	return true
}

func (g *Game) findPlayer(id int64) *Player {
	for _, v := range g.Players {
		if v.ID == id {
			return v
		}
	}

	return nil
}

func (g *Game) RemovePlayer(id int64) bool {
	g.Lock()
	defer g.Unlock()
	return g.removePlayer(id)
}

func (g *Game) removePlayer(id int64) bool {
	found := false
	numPlaying := 0
	for i, v := range g.Players {
		if v.ID == id {
			if !v.InGame {
				return false
			}

			// Don't remember more than 200 players, if this limit is reached its more than likely abuse
			if len(g.Players) > 200 {
				// Above limit, remove them entirely
				g.Players = append(g.Players[:i], g.Players[i+1:]...)
			} else {
				// Just mark them as not in game
				v.InGame = false
			}
			found = true
			continue
		}

		if v.InGame {
			numPlaying++
		}
	}

	if !found {
		return false
	}

	go g.sendAnnouncment(fmt.Sprintf("<@%d> Left the game (%d/%d)", id, numPlaying, g.PlayerLimit), false)

	if g.CurrentCardCzar == id && g.State != GameStatePreGame && g.State != GameStatePreRoundDelay {
		g.nextRound()
	}

	if g.GameMaster == id && numPlaying > 0 {
		for _, v := range g.Players {
			if v.InGame {
				g.GameMaster = v.ID
				go g.sendAnnouncment(fmt.Sprintf("GameMaster left, assigned <@%d> as new game master.", v.ID), false)
				break
			}
		}
	}

	return true
}

func (g *Game) setState(state GameState) {
	g.State = state
	g.StateEntered = time.Now()
}

func (g *Game) nextRound() {
	g.setState(GameStatePreRoundDelay)
}

func (g *Game) getRandomResponseCard() ResponseCard {
	if len(g.availableResponses) < 1 {
		g.loadPackResponses() // re-shuffle basically, TODO: exclude current hands
	}

	i := rand.Intn(len(g.availableResponses))
	card := g.availableResponses[i]
	g.availableResponses = append(g.availableResponses[:i], g.availableResponses[i+1:]...)

	// check whether a blank card was selected, return BlankCard if so
	if string(card) == "%blank" { // This is the string that signifies a blank card
		card = BlankCard
	}

	return card
}

func (g *Game) getRandomPlayerCards(num int) []ResponseCard {
	result := make([]ResponseCard, 0, num)

	if len(g.availableResponses) < 1 {
		return result
	}

	for len(result) < num {
		card := g.getRandomResponseCard()
		result = append(result, card)
	}

	return result
}

func (g *Game) sendAnnouncment(msg string, allPlayers bool) {
	embed := &discordgo.MessageEmbed{
		Description: msg,
	}

	if allPlayers {
		for _, v := range g.Players {
			go func(channel int64) {
				g.Session.ChannelMessageSendEmbed(channel, embed)
			}(v.Channel)
		}
	}

	g.Session.ChannelMessageSendEmbed(g.MasterChannel, embed)
}

func (g *Game) sendAnnouncmentMenu(msg string) {
	embed := &discordgo.MessageEmbed{
		Description: msg,
	}

	m, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: GetCommonCahButtons(),
	})
	if err == nil {
		g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
		g.LastMenuMessage = m.ID
	}
}

func (g *Game) Stop() {
	g.Lock()
	g.stop()
	g.Unlock()
}

func (g *Game) stop() {
	if g.stopped {
		return // Already stopped
	}

	g.stopped = true

	close(g.stopch)
}

func (g *Game) runTicker() {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.Println("CAH: cid: ", g.MasterChannel, " recovered from panic in running game!: ", r, "\n", stack)
			go g.Session.ChannelMessageSend(g.MasterChannel, "The running game almost caused a crash! The game has been terminated as a result. Contact the bot owner.")
			go g.Manager.RemoveGame(g.MasterChannel)
		}
	}()

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-g.stopch:
			return
		case <-ticker.C:
			g.Tick()
		}
	}
}

func (g *Game) Tick() {
	g.Lock()
	defer g.Unlock()

	expireAfter := GameExpireAfter
	if g.State == GameStatePreGame {
		expireAfter = GameExpireAfterPregame
	}
	if time.Since(g.LastAction) > expireAfter || g.numUsersInGame() < 1 {
		g.gameExpired()
		return
	}

	switch g.State {
	case GameStatePreGame:
		return
	case GameStatePreRoundDelay:
		if time.Since(g.StateEntered) > PreRoundDelayDuration {
			g.startRound()
			return
		}
	case GameStatePickingResponses:
		allPlayersDone := true
		oneResponsePicked := false
		for _, v := range g.Players {
			if v.ID == g.CurrentCardCzar || !v.PlayingThisRound() {
				continue
			}

			if !v.MadeSelections(g.CurrentPropmpt) {
				allPlayersDone = false
				if !v.sent15sWarning && time.Since(g.StateEntered) > (PickResponseDuration-(time.Second*15)) {
					v.sent15sWarning = true
					go g.Session.ChannelMessageSendEmbed(v.Channel, &discordgo.MessageEmbed{Description: "You have 15 seconds left"})
				}
			} else {
				oneResponsePicked = true
			}
		}

		if allPlayersDone || time.Since(g.StateEntered) >= PickResponseDuration {
			if oneResponsePicked {
				g.donePickingResponses()
			} else {
				// No one picked any cards...?
				g.sendAnnouncment("No one picked any cards, going to next round", false)
				g.nextRound()
			}
		}
	case GameStatePickingWinner:
		if !g.VoteMode {
			cardCzarPlayer := g.findPlayer(g.CurrentCardCzar)
			if cardCzarPlayer == nil {
				return
			}

			if !cardCzarPlayer.sent15sWarning && time.Since(g.StateEntered) > (PickWinnerDuration-(time.Second*15)) {
				cardCzarPlayer.sent15sWarning = true
				go g.Session.ChannelMessageSendEmbed(g.MasterChannel, &discordgo.MessageEmbed{Description: "You have 15 seconds left to pick a winner"})
			}
		}

		if time.Since(g.StateEntered) >= PickWinnerDuration {
			noVotes := true
			for _, v := range g.Players {
				if v.VotedFor != 0 {
					noVotes = false
					g.allVoted()
					break
				}
			}

			if noVotes {
				g.pickWinnerExpired()
			}
		}
	}

}

func (g *Game) numUsersInGame() int {
	num := 0
	for _, v := range g.Players {
		if v.InGame {
			num++
		}
	}

	return num
}

func (g *Game) startRound() {
	if g.numUsersInGame() < 2 {
		g.setState(GameStatePreGame)
		g.sendAnnouncment("Not enough players...", false)
		return
	}

	// Remove previous selected cards from players decks
	for _, v := range g.Responses {
		for _, sel := range v.Selections {
			for i, c := range v.Player.Cards {
				if c == sel {
					v.Player.Cards = append(v.Player.Cards[:i], v.Player.Cards[i+1:]...)
					break
				}
			}
		}
	}

	g.Responses = nil

	for _, v := range g.Players {
		v.SelectedCards = nil
		v.sent15sWarning = false
		v.VotedFor = 0
		v.ReceivedVotes = 0

		if v.InGame {
			v.Playing = true
		}
	}

	lastPick := 1
	if g.CurrentPropmpt != nil {
		lastPick = g.CurrentPropmpt.NumPick
	}

	// Pick random propmpt
	g.CurrentPropmpt = g.randomPrompt()

	// Give each player a random card (if they're below 10 cards)
	g.giveEveryoneCards(lastPick)

	if !g.VoteMode {
		// Pick next cardzar
		g.CurrentCardCzar = NextCardCzar(g.Players, g.CurrentCardCzar)
	}

	// Present the board
	g.presentStartRound()

	g.setState(GameStatePickingResponses)

}

func NextCardCzar(players []*Player, current int64) int64 {
	var next int64
	var lowest int64
	for _, v := range players {
		if v.ID == current || !v.PlayingThisRound() {
			continue
		}

		if v.ID > current && (v.ID < next || next == 0) {
			next = v.ID
		}

		if lowest == 0 || v.ID < lowest {
			lowest = v.ID
		}
	}

	if next == 0 {
		next = lowest
	}

	return next
}

func (g *Game) randomPrompt() *PromptCard {
	if len(g.availablePrompts) < 1 {
		g.loadPackPrompts() // ran out of cards, just relaod the packs
	}

	i := rand.Intn(len(g.availablePrompts))
	prompt := g.availablePrompts[i]
	g.availablePrompts = append(g.availablePrompts[:i], g.availablePrompts[i+1:]...)

	return prompt
}

func (g *Game) giveEveryoneCards(num int) {
	for _, p := range g.Players {
		if !p.PlayingThisRound() || len(p.Cards) >= 10 {
			continue
		}

		if !g.VoteMode && p.ID == g.CurrentCardCzar {
			continue
		}

		if num+len(p.Cards) > 10 {
			num = 10 - len(p.Cards)
		}

		cards := g.getRandomPlayerCards(num)
		p.Cards = append(p.Cards, cards...)
	}

}

func (g *Game) presentStartRound() {

	for _, player := range g.Players {
		go func(p *Player) {
			if !p.PlayingThisRound() {
				return
			}

			p.PresentBoard(g.Session, g.CurrentPropmpt, g.CurrentCardCzar)
		}(player)
	}

	instructions := fmt.Sprintf("Players: Check your dm for your cards and make your selections there, then return here, you have %d seconds", int(PickResponseDuration.Seconds()))
	if g.VoteMode {
		instructions += "\nAfter that you will all vote on the response"
	} else {
		instructions += "\nCardCzar: Wait until all players have picked card(s) then select the best one(s)"
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:  "Prompt",
			Value: g.CurrentPropmpt.PlaceHolder(),
		},
	}

	if !g.VoteMode {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "CardCzar",
			Value: fmt.Sprintf("<@%d>", g.CurrentCardCzar),
		})
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:  "Instructions",
		Value: instructions,
	})

	embed := &discordgo.MessageEmbed{
		Title: "Next round started!",
		Color: 7001855,
		// Description: msg,
		Fields: fields,
	}

	m, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: GetCommonCahButtons(),
	})
	if err == nil {
		g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
		g.LastMenuMessage = m.ID
	}
}

func (g *Game) donePickingResponses() {
	// Send a message to players that missed the round
	for _, v := range g.Players {
		if !v.PlayingThisRound() || (!g.VoteMode && v.ID == g.CurrentCardCzar) {
			continue
		}

		if len(v.SelectedCards) < g.CurrentPropmpt.NumPick {
			go g.Session.ChannelMessageSend(v.Channel, fmt.Sprintf("You didn't respond in time... winner is being picked in <#%d>", g.MasterChannel))
			v.SelectedCards = nil
			continue
		}

		selections := make([]ResponseCard, 0, len(v.SelectedCards))
		for _, sel := range v.SelectedCards {
			selections = append(selections, v.Cards[sel])
		}

		g.Responses = append(g.Responses, &PickedResonse{
			Player:     v,
			Selections: selections,
		})
	}

	// Shuffle them
	perm := rand.Perm(len(g.Responses))
	newResponses := make([]*PickedResonse, len(g.Responses))
	for i, v := range perm {
		newResponses[i] = g.Responses[v]
	}
	g.Responses = newResponses

	// Shows all the picks in both dm's and main channel
	g.setState(GameStatePickingWinner)
	g.presentPickedResponseCards(false)
}

func (g *Game) presentPickedResponseCards(edit bool) {

	desc := "Cards have been picked, "
	if g.VoteMode {
		desc += "you will now all vote on the best one, you cannot vote on your own selection."
	} else {
		desc += fmt.Sprintf("pick the best one(s) <@%d>!", g.CurrentCardCzar)
	}

	secondsLeft := int((PickWinnerDuration - time.Since(g.StateEntered)).Seconds())
	desc += fmt.Sprintf(" You have `%d` seconds.", secondsLeft)

	embed := &discordgo.MessageEmbed{
		Title:       "Pick the winner",
		Description: desc,
		Color:       5659830,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Prompt",
				Value: g.CurrentPropmpt.PlaceHolder(),
			},
			{
				Name: "Candidates",
			},
		},
	}

	currentEmbedField := embed.Fields[1]

	for i, v := range g.Responses {
		text := ""

		filledPrompt := g.CurrentPropmpt.WithCards(v.Selections)

		if g.VoteMode {
			text = fmt.Sprintf("%s: %s (`%d`) \n\n", CardSelectionEmojis[i], filledPrompt, v.Player.ReceivedVotes)
		} else {
			text = CardSelectionEmojis[i] + ": " + filledPrompt + "\n\n"
		}

		// Embed field values can be max 1024 in length, so make new fields as needed
		if currentEmbedField.Value != "" && utf8.RuneCountInString(currentEmbedField.Value)+utf8.RuneCountInString(text) > 1000 {
			newField := &discordgo.MessageEmbedField{
				Name:  "...",
				Value: text,
			}
			embed.Fields = append(embed.Fields, newField)
			currentEmbedField = newField
		} else {
			currentEmbedField.Value += text
		}
	}

	if g.VoteMode {
		remainingPlayersField := &discordgo.MessageEmbedField{
			Name: "Waiting for...",
		}

		for _, v := range g.Players {
			if v.PlayingThisRound() && v.VotedFor == 0 {
				if remainingPlayersField.Value != "" {
					remainingPlayersField.Value += ", "
				}
				remainingPlayersField.Value += "<@" + discordgo.StrID(v.ID) + ">"
			}
		}

		if remainingPlayersField.Value != "" {
			embed.Fields = append(embed.Fields, remainingPlayersField)
		}
	}

	voteOptions := []discordgo.SelectMenuOption{}
	for i := 0; i < len(g.Responses); i++ {
		selections := g.Responses[i].Selections
		chosenOption := selections[0]
		if len(selections) > 1 {
			for j := 1; j < len(selections); j++ {
				chosenOption += ", " + selections[j]
			}
		}
		option := discordgo.SelectMenuOption{
			Label: string(chosenOption),
			Value: CardSelectionEmojis[i],
		}
		if len(string(chosenOption)) > 100 {
			option.Label = string(chosenOption)[:100]
		}
		voteOptions = append(voteOptions, option)
	}

	voteOptionsComponent := []discordgo.TopLevelComponent{
		discordgo.ActionsRow{
			Components: []discordgo.InteractiveComponent{
				discordgo.SelectMenu{
					CustomID: CahCardSelectMenu,
					Options:  voteOptions,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.InteractiveComponent{
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: JoinEmoji},
					Style:    discordgo.SuccessButton,
					CustomID: CahGameJoined,
				},
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: LeaveEmoji},
					Style:    discordgo.DangerButton,
					CustomID: CahGameLeft,
				},
				discordgo.Button{
					Emoji:    &discordgo.ComponentEmoji{Name: PlayPauseEmoji},
					Style:    discordgo.PrimaryButton,
					CustomID: CahGamePlayPause,
				},
			},
		},
	}

	if edit {
		g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: voteOptionsComponent,
			Channel:    g.MasterChannel,
			ID:         g.LastMenuMessage,
		})
		return
	}

	msg, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: voteOptionsComponent,
	})
	if err != nil {
		return
	}

	g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
	g.LastMenuMessage = msg.ID
}

func (g *Game) pickWinnerExpired() {
	var content string
	if g.VoteMode {
		content = fmt.Sprintf("no one voted for anyone in %d seconds, skipping round...", int(PickWinnerDuration.Seconds()))
	} else {
		content = fmt.Sprintf("<@%d> didn't pick a winner in %d seconds, skipping round...", g.CurrentCardCzar, int(PickWinnerDuration.Seconds()))
	}

	_, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Content:    content,
		Components: GetCommonCahButtons(),
	})
	if err != nil {
		return
	}

	g.setState(GameStatePreRoundDelay)
}

func (g *Game) gameExpired() {
	g.Session.ChannelMessageSend(g.MasterChannel, "CAH Game expired, too long without any actions or no players.")
	go g.Manager.RemoveGame(g.MasterChannel)
	g.stop()
}

func (g *Game) removeOldInteractions(cID, mID int64) {
	if mID == 0 {
		return
	}
	g.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    cID,
		ID:         mID,
		Components: []discordgo.TopLevelComponent{},
	})
}

func (g *Game) HandleInteractionAdd(ic *discordgo.InteractionCreate) {
	g.Lock()
	defer g.Unlock()

	if g.State != GameStatePickingResponses {
		// Pong the interaction
		err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		if err != nil {
			logrus.WithError(err).Error("Failed Creating CAH Response")
			return
		}
	}

	if g.State == GameStateEnded {
		return
	}

	var user *discordgo.User
	if ic.Member != nil {
		user = ic.Member.User
	} else {
		user = ic.User
	}
	player := g.findPlayer(user.ID)
	if ic.Message.ID == g.LastMenuMessage {
		switch ic.MessageComponentData().CustomID {
		case CahGameJoined:
			if player != nil && player.InGame {
				return
			}

			go func() {
				username := ""

				if player == nil {
					username = user.Username
				} else {
					username = player.Username
				}

				if err := g.Manager.PlayerTryJoinGame(g.MasterChannel, user.ID, username); err == nil {
					g.Lock()
					g.LastAction = time.Now()
					g.Unlock()
				}
			}()

			return
		case CahGameLeft:
			go g.Manager.PlayerTryLeaveGame(user.ID)
			g.LastAction = time.Now()
			return
		case CahGamePlayPause:
			g.LastAction = time.Now()
			if g.State == GameStatePreGame && g.GameMaster == user.ID {
				g.setState(GameStatePreRoundDelay)
				go g.sendAnnouncment(fmt.Sprintf("Starting in %d seconds", int(PreRoundDelayDuration.Seconds())), false)
			} else if g.GameMaster == user.ID {
				for _, v := range g.Players {
					v.SelectedCards = nil
				}

				g.Responses = nil
				g.setState(GameStatePreGame)

				go g.sendAnnouncmentMenu(fmt.Sprintf("Paused, react with %s to continue, the game can be paused for max 30 minutes before it expires.", PlayPauseEmoji))
			}

			return
		default:
		}
	}

	// From here on out only players can take actions
	if player == nil {
		return
	}

	switch g.State {
	case GameStatePickingResponses:
		if ic.Message.ID != player.LastReactionMenu || ic.MessageComponentData().CustomID != CahCardSelectMenu {
			return
		}
		response := ic.MessageComponentData().Values[0]

		g.LastAction = time.Now()
		g.playerPickedResponseReaction(player, response, ic)
	case GameStatePickingWinner:
		if ic.Message.ID != g.LastMenuMessage || (player.ID != g.CurrentCardCzar && !g.VoteMode) || ic.MessageComponentData().CustomID != CahCardSelectMenu {
			return
		}

		emojiIndex := -1
		for i, v := range CardSelectionEmojis {
			if v == ic.MessageComponentData().Values[0] {
				emojiIndex = i
				break
			}
		}

		if emojiIndex == -1 || emojiIndex >= len(g.Responses) {
			return
		}

		if g.VoteMode {
			targetPlayer := g.Responses[emojiIndex].Player
			if targetPlayer.ID != player.ID && player.VotedFor == 0 {
				player.VotedFor = targetPlayer.ID
				targetPlayer.ReceivedVotes++
			}

			g.LastAction = time.Now()

			g.presentPickedResponseCards(true)
			for _, v := range g.Players {
				if v.PlayingThisRound() {
					if v.VotedFor == 0 {
						return
					}
				}
			}

			// Game is done
			g.allVoted()
		} else {
			winner := g.Responses[emojiIndex]
			winner.Player.Wins++
			g.presentWinners([]*PickedResonse{winner})

			if g.Players[0].Wins >= g.WinLimit {
				go g.Manager.RemoveGame(g.MasterChannel)
				g.setState(GameStateEnded)
			} else {
				g.setState(GameStatePreRoundDelay)
				g.LastAction = time.Now()
			}
		}
	}
}

func (g *Game) HandleMessageCreate(ic *discordgo.InteractionCreate) {
	g.Lock()
	defer g.Unlock()

	if ic.User == nil {
		// Only check dm messages
		return
	}

	cardResponse := ic.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	var player *Player
	for _, v := range g.Players {
		if v.ID == ic.User.ID {
			player = v
			break
		}
	}

	if player == nil || !player.FilingBlankCard {
		return
	}

	for _, v := range player.SelectedCards {
		card := player.Cards[v]
		if card == BlankCard {
			player.Cards[v] = ResponseCard(FilterEveryoneMentions(cardResponse))

			msg := "Selected **" + cardResponse + "**, "
			if len(player.SelectedCards) < g.CurrentPropmpt.NumPick {
				msg += fmt.Sprintf("select %d more card(s)", g.CurrentPropmpt.NumPick-len(player.SelectedCards))
			} else {
				msg += fmt.Sprintf("go to <#%d> and wait for the other players to finish their selections, the winner will be picked there", g.MasterChannel)
			}

			err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{{Description: msg}},
				},
			})
			if err != nil {
				return
			}
			break
		}
	}

	player.FilingBlankCard = false
}

const zeroWidthSpace = "\u200b"

var (
	mentionReplacer = strings.NewReplacer("@here", "@"+zeroWidthSpace+"here", "@everyone", "@"+zeroWidthSpace+"everyone")
)

func FilterEveryoneMentions(s string) string {
	s = mentionReplacer.Replace(s)
	return s
}

func (g *Game) allVoted() {

	winners := make([]*PickedResonse, 0)

	// Find the top votes
	winningVotes := 0
	for _, resp := range g.Responses {
		if resp.Player.ReceivedVotes > winningVotes {
			winningVotes = resp.Player.ReceivedVotes
		}
	}

	for _, resp := range g.Responses {
		if resp.Player.ReceivedVotes == winningVotes {
			winners = append(winners, resp)
			resp.Player.Wins++
		}
	}

	g.presentWinners(winners)

	if g.Players[0].Wins >= g.WinLimit {
		go g.Manager.RemoveGame(g.MasterChannel)
		g.setState(GameStateEnded)
	} else {
		g.setState(GameStatePreRoundDelay)
	}
}

func (g *Game) presentWinners(winningPicks []*PickedResonse) {

	// Sort the players by the number of wins
	// note: this wont change the cardzar order as thats done as lowest -> highest user ids
	sort.Slice(g.Players, func(i int, j int) bool {
		return g.Players[i].Wins > g.Players[j].Wins
	})

	wonFullGame := false
	if g.Players[0].Wins >= g.WinLimit {
		wonFullGame = true
	}

	wonFullGamePlayers := ""
	if wonFullGame {
		// There can be multiple people winning at the same time in vote mode
		for _, p := range g.Players {
			if p.Wins >= g.WinLimit {
				if wonFullGamePlayers != "" {
					wonFullGamePlayers += " and "
				}
				wonFullGamePlayers += p.Username
			}
		}
	}

	standings := "```\n"
	for _, v := range g.Players {
		standings += fmt.Sprintf("%-20s: %d\n", v.Username, v.Wins)
	}
	standings += "```"

	winnerCards := ""
	for i, v := range winningPicks {
		if i != 0 {
			winnerCards += "\n"
		}
		winnerCards += g.CurrentPropmpt.WithCards(v.Selections)
	}

	title := ""
	if !wonFullGame {
		winningPicksStr := ""
		for _, v := range winningPicks {
			if winningPicksStr != "" {
				winningPicksStr += " and "
			}
			winningPicksStr += "**" + v.Player.Username + "**"
		}

		title = fmt.Sprintf("%s won the round!", winningPicksStr)
	} else {
		title = fmt.Sprintf("%s WON THE ENTIRE GAME!!!", wonFullGamePlayers)
	}

	extraContent := ""
	if wonFullGame {
		extraContent = fmt.Sprintf("**ALL PRAISE %s OUR LORD(s) AND SAVIOUR(s)!**", wonFullGamePlayers)
	} else {
		extraContent = fmt.Sprintf("Next round in %d seconds...", int(PreRoundDelayDuration.Seconds()))
	}

	content := fmt.Sprintf("%s\n\n**Standings:**\n%s\n\n%s", winnerCards, standings, extraContent)
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: content,
		Color:       15276265,
	}

	msg, err := g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: GetCommonCahButtons(),
	})
	// msg, err := g.Session.ChannelMessageSend(g.MasterChannel, content)
	if err != nil {
		return
	}

	if !wonFullGame {
		g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
		g.LastMenuMessage = msg.ID
	}
}

func (g *Game) playerPickedResponseReaction(player *Player, response string, ic *discordgo.InteractionCreate) {
	if len(player.SelectedCards) >= g.CurrentPropmpt.NumPick {
		return
	}

	emojiIndex := -1
	for i, v := range CardSelectionEmojis {
		if v == response {
			emojiIndex = i
			break
		}
	}

	if emojiIndex < 0 {
		// Unknown reaction
		return
	}

	if emojiIndex >= len(player.Cards) {
		// Somehow picked a reaction that they cant (probably added the reaction themselv to mess with the bot)
		return
	}

	for _, selection := range player.SelectedCards {
		if selection == emojiIndex {
			// Already selected this card
			return
		}
	}

	card := player.Cards[emojiIndex]
	if card != BlankCard {
		// Pong the interaction
		err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		if err != nil {
			return
		}
	}

	respMsg := ""
	var showTextModal bool
	if card == BlankCard && player.FilingBlankCard {
		// Picked a blank card while already filing another blank card
		respMsg = "You're already filling in another blank card"
	} else {
		// Otherwise proceed normally
		player.SelectedCards = append(player.SelectedCards, emojiIndex)

		if card == BlankCard {
			respMsg += "Selected a blank card, type in your own response here now"
			player.FilingBlankCard = true
			showTextModal = true
		} else {
			respMsg = fmt.Sprintf("Selected **%s**", card)
			if len(player.SelectedCards) >= g.CurrentPropmpt.NumPick {
				respMsg += fmt.Sprintf(", go to <#%d> and wait for the other players to finish their selections, the winner will be picked there", g.MasterChannel)
			} else {
				respMsg += fmt.Sprintf(", select %d more cards", g.CurrentPropmpt.NumPick-len(player.SelectedCards))
			}
		}
	}
	if showTextModal {
		g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: CahBlankCardModal,
				Title:    "Blank Card",
				Components: []discordgo.TopLevelComponent{
					discordgo.ActionsRow{
						Components: []discordgo.InteractiveComponent{
							discordgo.TextInput{
								CustomID:  CahTextInput,
								Label:     "Enter your response",
								Style:     discordgo.TextInputShort,
								Required:  true,
								MaxLength: 200,
							},
						},
					},
				},
			},
		})
		return
	}
	go g.Session.ChannelMessageSendEmbed(player.Channel, &discordgo.MessageEmbed{Description: respMsg})
}

func (g *Game) loadFromSerializedState() {
	g.Lock()
	// update references
	for _, v := range g.Responses {
		for _, p := range g.Players {
			if v.Player.ID == p.ID {
				v.Player = p
				break
			}
		}
	}

	if !g.tickerRunning {
		go g.runTicker()
	}
	g.tickerRunning = true
	g.Unlock()

	go g.sendAnnouncment("Game has been fully loaded, it will still take another seconds before the game has resumed.", false)
}

type Player struct {
	ID              int64
	Username        string
	Cards           []ResponseCard
	SelectedCards   []int
	Wins            int
	FilingBlankCard bool
	VotedFor        int64
	ReceivedVotes   int

	Channel int64

	// Wether this user is playing this round, if the user joined in the middle of a round this will be false
	Playing bool
	InGame  bool
	Banned  bool

	sent15sWarning bool

	LastReactionMenu int64
}

func (p *Player) PlayingThisRound() bool {
	return p.Playing && p.InGame
}

func (p *Player) MadeSelections(currentPrompt *PromptCard) bool {
	if len(p.SelectedCards) < currentPrompt.NumPick {
		return false
	}

	if p.FilingBlankCard {
		return false
	}

	return true
}

func (p *Player) PresentBoard(session *discordgo.Session, currentPrompt *PromptCard, currentCardCzar int64) {
	if currentCardCzar == p.ID {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Pick %d card(s)!", currentPrompt.NumPick),
		Description: currentPrompt.PlaceHolder(),
	}

	options := []discordgo.SelectMenuOption{}

	for i, v := range p.Cards {
		cardValue := string(v)
		selectMenuOption := discordgo.SelectMenuOption{
			Value: CardSelectionEmojis[i],
		}
		if len(cardValue) > 100 {
			selectMenuOption.Label = cardValue[:100]
			if len(cardValue) > 200 {
				selectMenuOption.Description = cardValue[100:200]
			} else {
				selectMenuOption.Description = cardValue[100:]
			}
		} else {
			selectMenuOption.Label = cardValue
		}
		options = append(options, selectMenuOption)
	}

	resp, err := session.ChannelMessageSendComplex(p.Channel, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.TopLevelComponent{discordgo.ActionsRow{
			Components: []discordgo.InteractiveComponent{discordgo.SelectMenu{
				Options:  options,
				CustomID: CahCardSelectMenu,
			}},
		}},
	})
	if err != nil {
		logrus.WithError(err).Error("Failed sending CAH DM")
		return
	}

	p.removeLastMenuReactions(session)

	p.LastReactionMenu = resp.ID
}
