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

var TriviaDuration = time.Second * 30

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

type pickedOption struct {
	User   *discordgo.User
	Option int
}

type triviaSession struct {
	Manager         *triviaSessionManager
	GuildID         int64
	ChannelID       int64
	MessageID       int64
	Question        *TriviaQuestion
	SelectedOptions []*pickedOption
	createdAt       time.Time
	startedAt       time.Time
	ended           bool
	optionEmojis    []string

	mu sync.Mutex
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
	var optionEmojis []string
	if question.Type == "boolean" {
		optionEmojis = []string{
			"\U0001F1F9", // Regional ind. T
			"\U0001F1EB", // Regional ind. F
		}
	} else {
		optionEmojis = []string{
			"\U0001F1E6", // Regional ind. A
			"\U0001F1E7", // Regional ind. B
			"\U0001F1E8", // Regional ind. C
			"\U0001F1E9", // Regional ind. D
		}
	}

	session := &triviaSession{
		Manager:      tm,
		createdAt:    time.Now(),
		GuildID:      guildID,
		ChannelID:    channelID,
		Question:     question,
		optionEmojis: optionEmojis,
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
		t.startedAt = time.Now()
		t.updateMessage()
	}

	if time.Since(t.startedAt) > TriviaDuration {
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
		msgSend := &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.TopLevelComponent{discordgo.ActionsRow{Components: buttons}},
		}
		m, err = common.BotSession.ChannelMessageSendComplex(t.ChannelID, msgSend)
	} else {
		msgEdit := &discordgo.MessageEdit{
			Channel:    t.ChannelID,
			ID:         mID,
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: []discordgo.TopLevelComponent{discordgo.ActionsRow{Components: buttons}},
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

func (t *triviaSession) buildButtons() []discordgo.InteractiveComponent {
	components := []discordgo.InteractiveComponent{}
	if t.ended {
		for index, option := range t.Question.Options {
			totalAnswered := 0
			for _, v := range t.SelectedOptions {
				if v.Option == index {
					totalAnswered++
				}
			}
			style := discordgo.SuccessButton
			if option != t.Question.Answer {
				style = discordgo.SecondaryButton
			}
			button := discordgo.Button{
				Style:    style,
				Disabled: true,
				Label:    fmt.Sprintf("(%d)", totalAnswered),
				Emoji:    &discordgo.ComponentEmoji{Name: t.optionEmojis[index]},
				CustomID: option,
			}
			components = append(components, button)
		}
	} else {
		for index, option := range t.Question.Options {
			button := discordgo.Button{
				Style:    discordgo.PrimaryButton,
				Emoji:    &discordgo.ComponentEmoji{Name: t.optionEmojis[index]},
				CustomID: option,
			}
			components = append(components, button)
		}
	}

	return components
}

func (t *triviaSession) buildEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = fmt.Sprintf("Trivia Category: %s ", t.Question.Category)
	embed.Description += fmt.Sprintf("\n ## %s \n", t.Question.Question)

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text:    "Powered by Opentdb.com",
		IconURL: "https://opentdb.com/images/logo-banner.png",
	}

	optionsField := &discordgo.MessageEmbedField{
		Name:  "Options",
		Value: "",
	}

	for i, v := range t.Question.Options {
		if t.ended && v != t.Question.Answer {
			optionsField.Value += fmt.Sprintf("~~\n%s %s\n\n~~", t.optionEmojis[i], v)
		} else {
			optionsField.Value += fmt.Sprintf("** \n%s %s \n\n ** ", t.optionEmojis[i], v)
		}
	}

	embed.Fields = append(embed.Fields, optionsField)
	if !t.ended {
		timeLeft := t.startedAt.Add(TriviaDuration)
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Timer",
			Value: fmt.Sprintf("**Trivia ends <t:%d:R> \n**", timeLeft.Unix()),
		})
	}

	totalParticipants := len(t.SelectedOptions)
	if t.ended {
		field := &discordgo.MessageEmbedField{
			Name: "Time Up!",
		}

		winnerResponses := make([]*pickedOption, 0)
		for _, v := range t.SelectedOptions {
			if t.Question.Options[v.Option] == t.Question.Answer {
				winnerResponses = append(winnerResponses, v)
			}
		}

		totalWinners := len(winnerResponses)
		if totalParticipants == 0 {
			field.Value = "**No one participated :( \n **"
		} else if totalWinners == 0 {
			field.Value = fmt.Sprintf("**No Winners from %d participants! \n **", totalParticipants)
		} else {
			field.Value = fmt.Sprintf("**%d winners from %d participants! \n **", totalWinners, totalParticipants)
			if totalWinners > 20 {
				field.Value += "**First 20 winners: \n **"
				winnerResponses = winnerResponses[:20]
			}
			for _, v := range winnerResponses {
				field.Value += fmt.Sprintf("%s\n", v.User.Mention())
			}
		}
		embed.Fields = append(embed.Fields, field)
	} else if !t.ended && len(t.SelectedOptions) > 0 {
		field := &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("** \nTotal Participants : %d **", totalParticipants),
		}
		for i, v := range t.SelectedOptions {
			if i > 19 {
				field.Name = fmt.Sprintf("** \nTotal Participants : %d, Showing first 20 below **", totalParticipants)
				//show only the first 20 participants while trivia is in session and hasn't ended
				break
			}
			field.Value += fmt.Sprintf("\n%s", v.User.Mention())
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
	if t.ended || time.Since(t.startedAt) > TriviaDuration {
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

	if len(t.SelectedOptions) < 30 {
		t.updateMessage()
	}
	response.Type = discordgo.InteractionResponseDeferredMessageUpdate
	response.Data.Content = ""
	err = evt.Session.CreateInteractionResponse(ic.ID, ic.Token, &response)
	if err != nil {
		logger.WithError(err).Error("Failed creating interaction response")
	}
}
