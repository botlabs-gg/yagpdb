package trivia

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var triviaDuration = time.Second * 10

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, p.handleInteractionCreate, eventsystem.EventInteractionCreate)
}

func (p *Plugin) handleInteractionCreate(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	if ic.Type != discordgo.InteractionMessageComponent || ic.GuildID == 0 || ic.Member == nil || ic.Member.User.ID == common.BotUser.ID {
		return
	}

	manager.handleInteractionCreate(evt)
}

var manager = &triviaSessionManager{}

type triviaSessionManager struct {
	sessions []*triviaSession
	mu       sync.Mutex
}

var ErrSessionInChannel = errors.New("a trivia session already exists in this channel")

func (tm *triviaSessionManager) NewTrivia(guildID int64, channelID int64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, v := range tm.sessions {
		if v.ChannelID == channelID {
			return ErrSessionInChannel
		}
	}

	triviaQuestions, err := FetchQuestions(1)
	if err != nil {
		return err
	}

	question := triviaQuestions[0]
	session := &triviaSession{
		Manager:   tm,
		createdAt: time.Now(),
		GuildID:   guildID,
		ChannelID: channelID,
		Question:  question,
	}

	tm.sessions = append(tm.sessions, session)

	go session.tickLoop()

	return nil
}

func (t *triviaSessionManager) removeSession(session *triviaSession) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, v := range t.sessions {
		if v == session {
			t.sessions = append(t.sessions[:i], t.sessions[i+1:]...)
			break
		}
	}
}

func (tm *triviaSessionManager) handleInteractionCreate(evt *eventsystem.EventData) {
	tm.mu.Lock()
	ic := evt.InteractionCreate()
	for _, v := range tm.sessions {
		if v.ChannelID == ic.ChannelID {
			tm.mu.Unlock()
			v.mu.Lock()
			if v.MessageID == ic.Message.ID {
				v.mu.Unlock()
				v.handleInteractionAdd(evt)
				return
			}
			v.mu.Unlock()
			return
		}
	}

	tm.mu.Unlock()
}

type pickedOption struct {
	User   *discordgo.User
	Option int
}

type triviaSession struct {
	Manager           *triviaSessionManager
	GuildID           int64
	ChannelID         int64
	MessageID         int64
	Question          *TriviaQuestion
	SelectedOptions   []*pickedOption
	createdAt         time.Time
	startedAt         time.Time
	optionsRevealed   bool
	optionsRevealedAt time.Time
	ended             bool

	mu sync.Mutex
}

var optionEmojis = []string{
	"\U0001F1E6", // Regional ind. A
	"\U0001F1E7", // Regional ind. B
	"\U0001F1E8", // Regional ind. C
	"\U0001F1E9", // Regional ind. D
}

func (t *triviaSession) tickLoop() {
	for {
		ended := t.tick()
		if ended || time.Since(t.createdAt) > 1*time.Minute {
			t.Manager.removeSession(t)
			return
		}
		time.Sleep(time.Second)
	}
}

func (t *triviaSession) tick() (ended bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.startedAt.IsZero() {
		t.updateMessage()
		t.startedAt = time.Now()
	}

	if time.Since(t.startedAt) > time.Second*1 && !t.optionsRevealed {
		t.optionsRevealed = true
		t.optionsRevealedAt = time.Now()
		t.updateMessage()
	}

	if t.optionsRevealed && time.Since(t.optionsRevealedAt) > triviaDuration {
		t.ended = true
		t.updateMessage()
	}

	return t.ended
}

func (t *triviaSession) updateMessage() {
	embed := t.buildEmbed()
	buttons := t.buildButtons()

	mID := t.MessageID

	t.mu.Unlock()

	var err error
	var m *discordgo.Message
	if mID == 0 {
		m, err = common.BotSession.ChannelMessageSendEmbed(t.ChannelID, embed)
	} else {
		msgEdit := &discordgo.MessageEdit{
			Channel: t.ChannelID,
			ID:      mID,
			Embeds:  []*discordgo.MessageEmbed{embed},
		}

		if t.optionsRevealed && !t.ended {
			msgEdit.Components = []discordgo.MessageComponent{discordgo.ActionsRow{Components: buttons}}
		}
		if t.ended {
			msgEdit.Components = []discordgo.MessageComponent{}
		}
		_, err = common.BotSession.ChannelMessageEditComplex(msgEdit)
	}

	t.mu.Lock()

	if err != nil {
		logger.WithError(err).WithField("guild", t.GuildID).WithField("channel", t.ChannelID).Error("failed updating or sending trivia message")
	}

	if mID == 0 && err == nil {
		t.MessageID = m.ID
	}
}

func (t *triviaSession) buildButtons() []discordgo.MessageComponent {
	components := []discordgo.MessageComponent{}
	for index, option := range t.Question.Options {
		button := discordgo.Button{
			Style:    discordgo.PrimaryButton,
			Emoji:    &discordgo.ComponentEmoji{Name: optionEmojis[index]},
			CustomID: option,
		}
		components = append(components, button)
	}

	return components
}

func (t *triviaSession) buildEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}

	embed.Title = "Trivia!"

	embed.Description = fmt.Sprintf("**Category**: %s \n", t.Question.Category)
	embed.Description += fmt.Sprintf("\n**Question**: \n %s \n", t.Question.Question)

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: "Powered by Opentdb.com",
	}

	if t.optionsRevealed {

		optionsField := &discordgo.MessageEmbedField{
			Name: "Options:",
		}

		if t.ended {
			optionsField.Value += "~~"
		}

		for i, v := range t.Question.Options {
			optionsField.Value += optionEmojis[i] + " " + v + "\n\n"
		}

		if t.ended {
			optionsField.Value += "~~"
		}

		embed.Fields = append(embed.Fields, optionsField)
		if t.optionsRevealed && !t.ended {
			timeLeft := t.optionsRevealedAt.Add(triviaDuration)
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:  "Questions ends",
				Value: fmt.Sprintf("<t:%d:R>", timeLeft.Unix()),
			})
		}
	}

	if t.ended {

		correctAnswerIndex := -1
		for i, v := range t.Question.Options {
			if v == t.Question.Answer {
				correctAnswerIndex = i
			}
		}

		field := &discordgo.MessageEmbedField{
			Name:  "Time Up!",
			Value: fmt.Sprintf("Answer:  \n%s %s\n\n", optionEmojis[correctAnswerIndex], t.Question.Answer),
		}

		winnerResponses := make([]*pickedOption, 0)
		for _, v := range t.SelectedOptions {
			if t.Question.Options[v.Option] == t.Question.Answer {
				winnerResponses = append(winnerResponses, v)
			}
		}

		if len(winnerResponses) == 0 {
			field.Value += "**No one got the correct answer 😏 **"
		} else {
			field.Value += fmt.Sprintf("**Total %d winners!**\n", len(winnerResponses))
			if len(winnerResponses) > 20 {
				field.Value += "Fastest 20 winners are shown below: \n"
				winnerResponses = winnerResponses[:20]
			}
			for _, v := range winnerResponses {
				field.Value += fmt.Sprintf("%s\n", v.User.Mention())
			}
		}

		embed.Fields = append(embed.Fields, field)
	}

	return embed
}

func (t *triviaSession) handleInteractionAdd(evt *eventsystem.EventData) {
	ic := evt.InteractionCreate()
	ms, err := bot.GetMember(ic.GuildID, ic.Member.User.ID)
	if err != nil {
		logger.WithError(err).Error("Failed getting member from state for trivia interaction!")
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	response := discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: 64},
	}

	// Editing the embed can sometime get ratelimited
	if t.ended || time.Since(t.optionsRevealedAt) > triviaDuration {
		response.Data.Content = "You're too slow, trivia has already ended."
		err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &response)
		if err != nil {
			logger.WithError(err).Error("Failed creating interaction response")
		}
		return
	}

	// Check if user already picked a option
	for _, v := range t.SelectedOptions {
		if v.User.ID == ic.Member.User.ID {
			response.Data.Content = fmt.Sprintf("You've already picked an answer: `%s`, I am going to ignore this 😒", t.Question.Options[v.Option])
			err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &response)
			if err != nil {
				logger.WithError(err).Error("Failed creating interaction response")
			}
			return
		}
	}

	optionIndex := -1
	answer := ic.MessageComponentData()
	for i, v := range t.Question.Options {
		if answer.CustomID == v {
			optionIndex = i
			break
		}
	}

	t.SelectedOptions = append(t.SelectedOptions, &pickedOption{
		User:   &ms.User,
		Option: optionIndex,
	})

	response.Data.Content = fmt.Sprintf("I've recorded your answer as `%s`", answer.CustomID)
	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &response)
	if err != nil {
		logger.WithError(err).Error("Failed creating interaction response")
	}
}
