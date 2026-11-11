package cardsagainstdiscord

import (
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
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

	MinPlayers = 2
	// each response is labeled with one of the CardSelectionEmojis, so there can't be more players than those
	MaxPlayers = 11

	startingNotice = "Starting "
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

	// Players see their hand in an ephemeral message, the round is part of the ids so menus from previous rounds can be told apart
	CahShowCards       = "cah_show_cards"
	CahHandSelectMenu  = "cah_hand_select"
	ShowCardsButtonTxt = "Show my cards"
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

	PlayerLimit int
	WinLimit    int
	VoteMode    bool
	Packs       []string
	// Packs only available to this game, such as a server's own packs, kept with the game so they survive shard migration
	CustomPacks        []*CardPack
	availablePrompts   []*PromptCard
	availableResponses []ResponseCard

	Players []*Player

	State        GameState
	StateEntered time.Time

	// The time the most recent action was taken, if we go too long without a user action we expire the game
	LastAction time.Time

	CurrentPropmpt *PromptCard

	// Increased every round
	Round int

	// Zero uses the defaults, which is also the case for games from before these were configurable
	PickResponseTime time.Duration
	PickWinnerTime   time.Duration

	LastMenuMessage int64
	// the last menu without its buttons, used to remove them once a new menu is sent
	lastMenuDisplay *discordgo.Container
	lastMenuRows    []discordgo.TopLevelComponent
	// announcements are added to the last menu instead of being sent as their own messages
	menuNotices []string
	// edits of the menu are sent in the background, only the latest one is sent if several are waiting
	menuEditMu  sync.Mutex
	menuVersion atomic.Int64

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

// getRoundCahButtons are the common buttons along with the button players use to see their cards
func getRoundCahButtons() []discordgo.TopLevelComponent {
	buttons := GetCommonCahButtons()
	row := buttons[0].(discordgo.ActionsRow)
	row.Components = append(row.Components, discordgo.Button{
		Label:    ShowCardsButtonTxt,
		Style:    discordgo.SecondaryButton,
		CustomID: CahShowCards,
	})

	return []discordgo.TopLevelComponent{row}
}

func (g *Game) Created() error {
	g.LastAction = time.Now()

	container := textContainer(0,
		fmt.Sprintf("## <@%d> started Cards Against Humanity", g.GameMaster),
		"**Packs**\n"+common.FormatList(g.Packs, "and"),
		fmt.Sprintf("Press %s to join and %s to leave, the game master starts and pauses the game with %s", JoinEmoji, LeaveEmoji, PlayPauseEmoji),
	)
	if err := g.sendMenu(container, GetCommonCahButtons()...); err != nil {
		return err
	}

	g.stopch = make(chan bool)

	g.loadPackPrompts()
	g.loadPackResponses()

	go g.runTicker()

	g.tickerRunning = true
	return nil
}

func (g *Game) resolvePack(name string) *CardPack {
	for _, v := range g.CustomPacks {
		if v.Name == name {
			return v
		}
	}

	return Packs[name]
}

func (g *Game) loadPackResponses() {
	for _, v := range g.Packs {
		if pack := g.resolvePack(v); pack != nil {
			g.availableResponses = append(g.availableResponses, pack.Responses...)
		}
	}
}
func (g *Game) loadPackPrompts() {
	for _, v := range g.Packs {
		if pack := g.resolvePack(v); pack != nil {
			g.availablePrompts = append(g.availablePrompts, pack.Prompts...)
		}
	}
}

func (g *Game) pickResponseTime() time.Duration {
	if g.PickResponseTime > 0 {
		return g.PickResponseTime
	}
	return PickResponseDuration
}

func (g *Game) pickWinnerTime() time.Duration {
	if g.PickWinnerTime > 0 {
		return g.PickWinnerTime
	}
	return PickWinnerDuration
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

	existing := g.findPlayer(id)
	if existing != nil {

		if existing.Banned {
			return false
		}

		existing.InGame = true
		msg = "Came back"
	} else {
		p := &Player{
			ID:       id,
			Username: username,
			Cards:    g.getRandomPlayerCards(8),
			InGame:   true,
		}

		g.Players = append(g.Players, p)

		msg = "Joined the game"
	}

	g.sendAnnouncment(fmt.Sprintf("<@%d> %s! (%d/%d)", id, msg, numPlaying+1, g.PlayerLimit))
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

	g.sendAnnouncment(fmt.Sprintf("<@%d> Left the game (%d/%d)", id, numPlaying, g.PlayerLimit))

	if g.CurrentCardCzar == id && !g.VoteMode && g.State != GameStatePreGame && g.State != GameStatePreRoundDelay {
		g.sendAnnouncment("The card czar left, skipping this round.")
		g.nextRound()
	}

	if g.GameMaster == id && numPlaying > 0 {
		for _, v := range g.Players {
			if v.InGame {
				g.GameMaster = v.ID
				g.sendAnnouncment(fmt.Sprintf("GameMaster left, assigned <@%d> as new game master.", v.ID))
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

	for len(result) < num {
		if len(g.availableResponses) < 1 {
			// re-shuffle the deck once it runs out, which can't help if the packs have no responses at all
			g.loadPackResponses()
			if len(g.availableResponses) < 1 {
				break
			}
		}

		result = append(result, g.getRandomResponseCard())
	}

	return result
}

// sendAnnouncment adds the message to the last menu, or sends it on its own when there's no menu to add it to
// assumes the game is locked
func (g *Game) sendAnnouncment(msg string) {
	if g.LastMenuMessage == 0 || g.lastMenuDisplay == nil {
		go g.sendContainer(textContainer(0, msg))
		return
	}

	g.menuNotices = append(g.menuNotices, msg)
	if len(g.menuNotices) > maxMenuNotices {
		g.menuNotices = g.menuNotices[len(g.menuNotices)-maxMenuNotices:]
	}

	g.editMenu()
}

func (g *Game) sendAnnouncmentMenu(msg string) {
	g.sendMenu(textContainer(0, msg), GetCommonCahButtons()...)
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
		var warn []int64
		for _, v := range g.Players {
			if v.ID == g.CurrentCardCzar || !v.PlayingThisRound() {
				continue
			}

			if !v.MadeSelections(g.CurrentPropmpt) {
				allPlayersDone = false
				if !v.sent15sWarning && time.Since(g.StateEntered) > (g.pickResponseTime()-(time.Second*15)) {
					v.sent15sWarning = true
					warn = append(warn, v.ID)
				}
			} else {
				oneResponsePicked = true
			}
		}

		if len(warn) > 0 {
			go g.sendPickWarning(warn, g.StateEntered.Add(g.pickResponseTime()))
		}

		if allPlayersDone || time.Since(g.StateEntered) >= g.pickResponseTime() {
			if oneResponsePicked {
				g.donePickingResponses()
			} else {
				// No one picked any cards...?
				g.sendAnnouncment("No one picked any cards, going to next round")
				g.nextRound()
			}
		}
	case GameStatePickingWinner:
		if !g.VoteMode {
			cardCzarPlayer := g.findPlayer(g.CurrentCardCzar)
			if cardCzarPlayer == nil {
				return
			}

			if !cardCzarPlayer.sent15sWarning && time.Since(g.StateEntered) > (g.pickWinnerTime()-(time.Second*15)) {
				cardCzarPlayer.sent15sWarning = true
				go g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
					Components:      []discordgo.TopLevelComponent{textContainer(0, fmt.Sprintf("<@%d> time to pick a winner runs out %s", g.CurrentCardCzar, relativeTime(g.StateEntered.Add(g.pickWinnerTime()))))},
					Flags:           discordgo.MessageFlagsIsComponentsV2,
					AllowedMentions: discordgo.AllowedMentions{Users: []int64{g.CurrentCardCzar}},
				})
			}
		}

		if time.Since(g.StateEntered) >= g.pickWinnerTime() {
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

func (g *Game) sendPickWarning(players []int64, deadline time.Time) {
	var mentions strings.Builder
	for _, id := range players {
		fmt.Fprintf(&mentions, "<@%d> ", id)
	}

	g.Session.ChannelMessageSendComplex(g.MasterChannel, &discordgo.MessageSend{
		Components:      []discordgo.TopLevelComponent{textContainer(0, fmt.Sprintf("%stime to pick your cards runs out %s, press **%s** on the round message", mentions.String(), relativeTime(deadline), ShowCardsButtonTxt))},
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		AllowedMentions: discordgo.AllowedMentions{Users: players},
	})
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
	if g.numUsersInGame() < MinPlayers {
		// players left during the countdown, which no longer applies
		g.removeNotices(startingNotice)
		g.setState(GameStatePreGame)
		g.sendAnnouncment("Not enough players to start, waiting for more players to join.")
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
	g.Round++

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

		cards := g.getRandomPlayerCards(min(num, 10-len(p.Cards)))
		p.Cards = append(p.Cards, cards...)
	}

}

func (g *Game) presentStartRound() {
	// the round is presented right before picking starts
	instructions := fmt.Sprintf("Players: Press **%s** to see your cards and make your selections, time runs out %s", ShowCardsButtonTxt, relativeTime(time.Now().Add(g.pickResponseTime())))
	if g.VoteMode {
		instructions += "\nAfter that you will all vote on the response"
	} else {
		instructions += "\nCardCzar: Wait until all players have picked card(s) then select the best one(s)"
	}

	cardCzar := ""
	if !g.VoteMode {
		cardCzar = fmt.Sprintf("**CardCzar**\n<@%d>", g.CurrentCardCzar)
	}

	g.sendMenu(textContainer(colorRoundStarted,
		"## Next round started!",
		"**Prompt**\n"+g.CurrentPropmpt.PlaceHolder(),
		cardCzar,
		"**Instructions**\n"+instructions,
	), getRoundCahButtons()...)
}

func (g *Game) donePickingResponses() {
	// Send a message to players that missed the round
	for _, v := range g.Players {
		if !v.PlayingThisRound() || (!g.VoteMode && v.ID == g.CurrentCardCzar) {
			continue
		}

		if len(v.SelectedCards) < g.CurrentPropmpt.NumPick {
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

// candidatesTextLimit keeps the candidates well under the 4000 characters of text a components v2 message can have
const candidatesTextLimit = 2000

func (g *Game) presentPickedResponseCards(edit bool) {

	desc := "Cards have been picked, "
	if g.VoteMode {
		desc += "you will now all vote on the best one, you cannot vote on your own selection."
	} else {
		desc += fmt.Sprintf("pick the best one(s) <@%d>!", g.CurrentCardCzar)
	}

	desc += " Time runs out " + relativeTime(g.StateEntered.Add(g.pickWinnerTime())) + "."

	maxCandidateLen := candidatesTextLimit
	if len(g.Responses) > 0 {
		maxCandidateLen = candidatesTextLimit / len(g.Responses)
	}

	var candidates strings.Builder
	for i, v := range g.Responses {
		filledPrompt := common.CutStringShort(g.CurrentPropmpt.WithCards(v.Selections), maxCandidateLen)

		if g.VoteMode {
			fmt.Fprintf(&candidates, "%s: %s (`%d`) \n\n", CardSelectionEmojis[i], filledPrompt, v.Player.ReceivedVotes)
		} else {
			candidates.WriteString(CardSelectionEmojis[i] + ": " + filledPrompt + "\n\n")
		}
	}

	waitingFor := ""
	if g.VoteMode {
		var remaining []string
		for _, v := range g.Players {
			if g.waitingForVote(v) {
				remaining = append(remaining, "<@"+discordgo.StrID(v.ID)+">")
			}
		}

		if len(remaining) > 0 {
			waitingFor = "**Waiting for...**\n" + strings.Join(remaining, ", ")
		}
	}

	container := textContainer(colorPickWinner,
		"## Pick the winner\n"+desc,
		"**Prompt**\n"+g.CurrentPropmpt.PlaceHolder(),
		"**Candidates**\n"+candidates.String(),
		waitingFor,
	)

	voteOptions := []discordgo.SelectMenuOption{}
	for i := 0; i < len(g.Responses); i++ {
		selections := g.Responses[i].Selections
		chosenOption := selections[0]
		if len(selections) > 1 {
			for j := 1; j < len(selections); j++ {
				chosenOption += ", " + selections[j]
			}
		}
		label, _ := splitOptionText(string(chosenOption))
		voteOptions = append(voteOptions, discordgo.SelectMenuOption{
			Label: label,
			Value: CardSelectionEmojis[i],
		})
	}

	rows := append([]discordgo.TopLevelComponent{
		discordgo.ActionsRow{
			Components: []discordgo.InteractiveComponent{
				discordgo.SelectMenu{
					CustomID: CahCardSelectMenu,
					Options:  voteOptions,
				},
			},
		},
	}, GetCommonCahButtons()...)

	if edit {
		g.lastMenuDisplay = &container
		g.lastMenuRows = rows
		g.editMenu()
		return
	}

	g.sendMenu(container, rows...)
}

func (g *Game) pickWinnerExpired() {
	var content string
	if g.VoteMode {
		content = fmt.Sprintf("no one voted for anyone in %d seconds, skipping round...", int(g.pickWinnerTime().Seconds()))
	} else {
		content = fmt.Sprintf("<@%d> didn't pick a winner in %d seconds, skipping round...", g.CurrentCardCzar, int(g.pickWinnerTime().Seconds()))
	}

	g.sendMenu(textContainer(0, content), GetCommonCahButtons()...)
	g.setState(GameStatePreRoundDelay)
}

func (g *Game) gameExpired() {
	g.Session.ChannelMessageSend(g.MasterChannel, "CAH Game expired, too long without any actions or no players.")
	go g.Manager.RemoveGame(g.MasterChannel)
	g.stop()
}

func (g *Game) HandleInteractionAdd(ic *discordgo.InteractionCreate) {
	g.Lock()
	defer g.Unlock()

	var user *discordgo.User
	if ic.Member != nil {
		user = ic.Member.User
	} else {
		user = ic.User
	}
	player := g.findPlayer(user.ID)

	if ic.Type == discordgo.InteractionModalSubmit {
		g.handleBlankCardSubmit(player, ic)
		return
	}

	customID := ic.MessageComponentData().CustomID
	if customID == CahShowCards || strings.HasPrefix(customID, CahHandSelectMenu) {
		g.handleHandInteraction(player, ic, customID)
		return
	}

	// Pong the interaction, the rest are on the public game messages which are sent or edited separately
	err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed Creating CAH Response")
		return
	}

	if g.State == GameStateEnded {
		g.reply(ic, "This game is over.")
		return
	}

	if ic.Message.ID != g.LastMenuMessage {
		g.reply(ic, "This menu is no longer active, use the latest game message.")
		return
	}

	switch customID {
	case CahGameJoined:
		g.handleJoinClick(ic, user, player)
	case CahGameLeft:
		if player == nil || !player.InGame {
			g.reply(ic, "You're not in this game.")
			return
		}

		g.LastAction = time.Now()
		go func() {
			if err := g.Manager.PlayerTryLeaveGame(user.ID); err != nil {
				g.reply(ic, "You're not in this game.")
				return
			}
			g.reply(ic, "You left the game.")
		}()
	case CahGamePlayPause:
		g.handlePlayPauseClick(ic, user)
	case CahCardSelectMenu:
		g.handleWinnerPick(ic, player)
	}
}

// reply sends a message only the user that used the component sees, the interaction has to be acknowledged already
func (g *Game) reply(ic *discordgo.InteractionCreate, msg string) {
	go func() {
		_, err := g.Session.CreateFollowupMessage(ic.ApplicationID, ic.Token, &discordgo.WebhookParams{
			Content:         msg,
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.AllowedMentions{},
		})
		if err != nil {
			logrus.WithError(err).Error("Failed sending CAH reply")
		}
	}()
}

func (g *Game) handleJoinClick(ic *discordgo.InteractionCreate, user *discordgo.User, player *Player) {
	if player != nil && player.InGame {
		g.reply(ic, "You're already in this game.")
		return
	}

	username := user.Username
	if player != nil {
		username = player.Username
	}

	// joining goes through the manager, which locks this game itself
	go func() {
		err := g.Manager.PlayerTryJoinGame(g.MasterChannel, user.ID, username)
		switch err {
		case nil:
			g.Lock()
			g.LastAction = time.Now()
			playing := g.State == GameStatePreGame || g.State == GameStatePreRoundDelay
			g.Unlock()

			if playing {
				g.reply(ic, "You joined the game!")
			} else {
				g.reply(ic, "You joined the game! You'll get cards from the next round.")
			}
		case ErrPlayerAlreadyInGame:
			g.reply(ic, "You're already in a game in another channel, leave it first.")
		case ErrGameFull:
			g.reply(ic, "You can't join this game, it's full.")
		default:
			g.reply(ic, "This game is no longer running.")
		}
	}()
}

func (g *Game) handlePlayPauseClick(ic *discordgo.InteractionCreate, user *discordgo.User) {
	if g.GameMaster != user.ID {
		g.reply(ic, fmt.Sprintf("Only the game master <@%d> can start or pause the game.", g.GameMaster))
		return
	}

	g.LastAction = time.Now()
	if g.State == GameStatePreGame {
		if missing := MinPlayers - g.numUsersInGame(); missing > 0 {
			g.reply(ic, fmt.Sprintf("At least %d players are needed to start, waiting for %d more.", MinPlayers, missing))
			return
		}

		g.setState(GameStatePreRoundDelay)
		g.sendAnnouncment(startingNotice + relativeTime(g.StateEntered.Add(PreRoundDelayDuration)))
		g.reply(ic, "Starting the game!")
		return
	}

	for _, v := range g.Players {
		v.SelectedCards = nil
	}

	g.Responses = nil
	g.setState(GameStatePreGame)

	g.sendAnnouncmentMenu(fmt.Sprintf("Paused, press %s to continue. The game ends %s if it isn't continued.", PlayPauseEmoji, relativeTime(time.Now().Add(GameExpireAfterPregame))))
	g.reply(ic, "Paused the game.")
}

func (g *Game) handleWinnerPick(ic *discordgo.InteractionCreate, player *Player) {
	if g.State != GameStatePickingWinner {
		g.reply(ic, "The winner isn't being picked right now.")
		return
	}

	if player == nil || !player.PlayingThisRound() {
		g.reply(ic, "Only players of this round can pick the winner.")
		return
	}

	if !g.VoteMode && player.ID != g.CurrentCardCzar {
		g.reply(ic, fmt.Sprintf("Only the card czar <@%d> can pick the winner.", g.CurrentCardCzar))
		return
	}

	emojiIndex := slices.Index(CardSelectionEmojis, ic.MessageComponentData().Values[0])
	if emojiIndex == -1 || emojiIndex >= len(g.Responses) {
		return
	}

	picked := g.Responses[emojiIndex]
	if !g.VoteMode {
		g.reply(ic, "You picked "+g.CurrentPropmpt.WithCards(picked.Selections))

		picked.Player.Wins++
		g.presentWinners([]*PickedResonse{picked})

		if g.Players[0].Wins >= g.WinLimit {
			go g.Manager.RemoveGame(g.MasterChannel)
			g.setState(GameStateEnded)
		} else {
			g.setState(GameStatePreRoundDelay)
			g.LastAction = time.Now()
		}
		return
	}

	switch {
	case player.VotedFor != 0:
		g.reply(ic, "You already voted this round.")
		return
	case picked.Player.ID == player.ID:
		g.reply(ic, "You can't vote for your own cards.")
		return
	}

	player.VotedFor = picked.Player.ID
	picked.Player.ReceivedVotes++
	g.LastAction = time.Now()
	g.reply(ic, "You voted for "+g.CurrentPropmpt.WithCards(picked.Selections))

	g.presentPickedResponseCards(true)
	for _, v := range g.Players {
		if g.waitingForVote(v) {
			return
		}
	}

	// everyone voted
	g.allVoted()
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

	var standings strings.Builder
	standings.WriteString("```\n")
	for _, v := range g.Players {
		standings.WriteString(fmt.Sprintf("%-20s: %d\n", v.Username, v.Wins))
	}
	standings.WriteString("```")

	var winnerCards strings.Builder
	for i, v := range winningPicks {
		if i != 0 {
			winnerCards.WriteString("\n")
		}
		winnerCards.WriteString(g.CurrentPropmpt.WithCards(v.Selections))
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
		// the next round's delay starts right after the winners are presented
		extraContent = "Next round starts " + relativeTime(time.Now().Add(PreRoundDelayDuration))
	}

	container := textContainer(colorWinners, "## "+title+"\n"+winnerCards.String(), "**Standings:**\n"+standings.String(), extraContent)
	if !wonFullGame {
		g.sendMenu(container, GetCommonCahButtons()...)
		return
	}

	// the game is over, so the final message has no buttons
	g.removeOldInteractions(g.MasterChannel, g.LastMenuMessage)
	g.sendContainer(container)
}

func (g *Game) handMenuID() string {
	return fmt.Sprintf("%s:%d", CahHandSelectMenu, g.Round)
}

// handUnavailableReason returns why the player can't pick cards right now, or an empty string if they can
func (g *Game) handUnavailableReason(player *Player) string {
	switch {
	case player == nil || !player.InGame:
		return fmt.Sprintf("You're not in this game, press %s on the game message to join.", JoinEmoji)
	case g.State != GameStatePickingResponses:
		return "Cards can only be picked while a round is running, wait for the next round."
	case !player.Playing:
		return "You joined during this round, you'll get to play from the next round."
	case !g.VoteMode && player.ID == g.CurrentCardCzar:
		return "You're the card czar this round, wait for the others to pick their cards and then choose the winner."
	}

	return ""
}

// handleHandInteraction handles players opening their hand and picking cards from it, all of which happens in an ephemeral message
func (g *Game) handleHandInteraction(player *Player, ic *discordgo.InteractionCreate, customID string) {
	fromHand := customID != CahShowCards

	if reason := g.handUnavailableReason(player); reason != "" {
		g.respondHand(ic, fromHand, textContainer(0, reason), nil)
		return
	}

	if fromHand {
		if customID != g.handMenuID() {
			g.respondHand(ic, fromHand, textContainer(0, fmt.Sprintf("That round is over, press **%s** on the latest round message.", ShowCardsButtonTxt)), nil)
			return
		}

		g.LastAction = time.Now()
		if g.playerPickedCard(player, ic.MessageComponentData().Values[0], ic) {
			// a modal was opened for a blank card instead
			return
		}
	}

	container, rows := g.handMessage(player)
	g.respondHand(ic, fromHand, container, rows)
}

// respondHand sends a new ephemeral message, or updates the ephemeral message the hand menu is in
func (g *Game) respondHand(ic *discordgo.InteractionCreate, update bool, container discordgo.Container, rows []discordgo.TopLevelComponent) {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: withRows(container, rows),
			Flags:      discordgo.MessageFlagsEphemeral | discordgo.MessageFlagsIsComponentsV2,
		},
	}

	if update {
		// the flags of the message being updated stay as they are
		resp.Type = discordgo.InteractionResponseUpdateMessage
		resp.Data.Flags = 0
	}

	err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, resp)
	if err != nil {
		logrus.WithError(err).Error("Failed responding with CAH hand")
	}
}

func (g *Game) handMessage(player *Player) (discordgo.Container, []discordgo.TopLevelComponent) {
	picked := ""
	if len(player.SelectedCards) > 0 {
		var sb strings.Builder
		sb.WriteString("**Picked**\n")
		for _, i := range player.SelectedCards {
			fmt.Fprintf(&sb, "**%s**\n", player.Cards[i])
		}
		picked = sb.String()
	}

	header := fmt.Sprintf("## Pick %d card(s)!\n%s", g.CurrentPropmpt.NumPick, g.CurrentPropmpt.PlaceHolder())

	if player.MadeSelections(g.CurrentPropmpt) {
		return textContainer(0, header, picked, "-# Waiting for the other players to finish, the winner will be picked in the channel."), nil
	}

	container := textContainer(0, header, picked, fmt.Sprintf("-# Pick %d more card(s)", g.CurrentPropmpt.NumPick-len(player.SelectedCards)))

	options := []discordgo.SelectMenuOption{}
	for i, v := range player.Cards {
		if slices.Contains(player.SelectedCards, i) {
			continue
		}

		label, description := splitOptionText(string(v))
		options = append(options, discordgo.SelectMenuOption{
			Label:       label,
			Description: description,
			Value:       CardSelectionEmojis[i],
		})
	}

	if len(options) == 0 {
		// select menus need at least one option
		return textContainer(0, header, picked, "-# You don't have any cards left to pick, wait for the next round."), nil
	}

	return container, []discordgo.TopLevelComponent{discordgo.ActionsRow{
		Components: []discordgo.InteractiveComponent{discordgo.SelectMenu{
			Options:  options,
			CustomID: g.handMenuID(),
		}},
	}}
}

func (g *Game) playerPickedCard(player *Player, value string, ic *discordgo.InteractionCreate) bool {
	if len(player.SelectedCards) >= g.CurrentPropmpt.NumPick {
		return false
	}

	index := slices.Index(CardSelectionEmojis, value)
	if index < 0 || index >= len(player.Cards) || slices.Contains(player.SelectedCards, index) {
		return false
	}

	if player.Cards[index] != BlankCard {
		player.SelectedCards = append(player.SelectedCards, index)
		return false
	}

	// the blank card is only selected once the modal is submitted, so dismissing it lets them pick it again
	err := g.Session.CreateInteractionResponse(ic.ID, ic.Token, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("%s:%d:%d", CahBlankCardModal, g.Round, index),
			Title:    "Blank Card",
			Components: []discordgo.TopLevelComponent{
				discordgo.Label{
					Label: "Enter your response",
					Component: discordgo.TextInput{
						CustomID:  CahTextInput,
						Style:     discordgo.TextInputShort,
						Required:  true,
						MaxLength: 200,
					},
				},
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("Failed opening CAH blank card modal")
	}

	return true
}

func (g *Game) handleBlankCardSubmit(player *Player, ic *discordgo.InteractionCreate) {
	data := ic.ModalSubmitData()

	var round, index int
	_, err := fmt.Sscanf(data.CustomID, CahBlankCardModal+":%d:%d", &round, &index)
	if err != nil {
		return
	}

	if reason := g.handUnavailableReason(player); reason != "" {
		g.respondHand(ic, true, textContainer(0, reason), nil)
		return
	}

	if round != g.Round {
		g.respondHand(ic, true, textContainer(0, "That round is over, your blank card wasn't used."), nil)
		return
	}

	text, _ := data.TextInputValue(CahTextInput)
	text = strings.TrimSpace(text)
	if text != "" && index < len(player.Cards) && player.Cards[index] == BlankCard &&
		!slices.Contains(player.SelectedCards, index) && len(player.SelectedCards) < g.CurrentPropmpt.NumPick {

		player.Cards[index] = ResponseCard(FilterEveryoneMentions(text))
		player.SelectedCards = append(player.SelectedCards, index)
		g.LastAction = time.Now()
	}

	container, rows := g.handMessage(player)
	g.respondHand(ic, true, container, rows)
}

func (g *Game) loadFromSerializedState() {
	g.Lock()
	// the decks aren't serialized
	g.loadPackPrompts()
	g.loadPackResponses()

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
	g.sendAnnouncment("Game has been fully loaded, it will still take another seconds before the game has resumed.")
	g.Unlock()
}

type Player struct {
	ID            int64
	Username      string
	Cards         []ResponseCard
	SelectedCards []int
	Wins          int
	VotedFor      int64
	ReceivedVotes int

	// Wether this user is playing this round, if the user joined in the middle of a round this will be false
	Playing bool
	InGame  bool
	Banned  bool

	sent15sWarning bool
}

func (p *Player) PlayingThisRound() bool {
	return p.Playing && p.InGame
}

func (p *Player) MadeSelections(currentPrompt *PromptCard) bool {
	return len(p.SelectedCards) >= currentPrompt.NumPick
}
