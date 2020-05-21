package rsvp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	"github.com/jonas747/yagpdb/rsvp/models"
	"github.com/jonas747/yagpdb/timezonecompanion"
	"github.com/volatiletech/sqlboiler/boil"
)

type SetupState int

const (
	SetupStateChannel SetupState = iota
	SetupStateTitle
	SetupStateMaxParticipants
	SetupStateWhen
	SetupStateWhenConfirm
)

type SetupSession struct {
	mu     sync.Mutex
	plugin *Plugin

	setupMessages []int64

	CreatedOnMessageID int64
	GuildID            int64
	AuthorID           int64
	SetupChannel       int64

	State SetupState

	MaxParticipants int
	Title           string
	Channel         int64
	When            time.Time

	LastAction time.Time
	stopCH     chan bool
	stopped    bool
}

func (s *SetupSession) handleMessage(m *discordgo.Message) {
	if s.CreatedOnMessageID == m.ID {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.setupMessages = append(s.setupMessages, m.ID)

	s.LastAction = time.Now()

	if strings.EqualFold(m.Content, "exit") || strings.EqualFold(m.Content, "stop") {
		s.sendMessage("RSVP Event setup cancelled")
		go s.remove()
		return
	}

	switch s.State {
	case SetupStateChannel:
		s.handleMessageSetupStateChannel(m)
	case SetupStateTitle:
		s.handleMessageSetupStateTitle(m)
	case SetupStateMaxParticipants:
		s.handleMessageSetupStateMaxParticipants(m)
	case SetupStateWhen:
		s.handleMessageSetupStateWhen(m)
	case SetupStateWhenConfirm:
		s.handleMessageSetupStateWhenConfirm(m)
	}
}

func (s *SetupSession) handleMessageSetupStateChannel(m *discordgo.Message) {
	targetChannel := int64(0)

	gs := bot.State.Guild(true, m.GuildID)
	if gs == nil {
		logger.WithField("guild", m.GuildID).Error("Guild not found")
		return
	}

	if strings.EqualFold(m.Content, "here") || strings.EqualFold(m.Content, "this") || strings.EqualFold(m.Content, "this one") {
		// current channel
		targetChannel = s.SetupChannel
	} else if strings.HasPrefix(m.Content, "<#") && strings.HasSuffix(m.Content, ">") {
		// channel mention
		idStr := m.Content[2 : len(m.Content)-1]
		if parsed, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			if gs.Channel(true, parsed) != nil {
				targetChannel = parsed
			}
		}
	} else {
		// search by name
		nameSearch := strings.ReplaceAll(m.Content, " ", "-")
		gs.RLock()
		for _, v := range gs.Channels {
			if strings.EqualFold(v.Name, nameSearch) {
				targetChannel = v.ID
				break
			}
		}
		gs.RUnlock()
	}

	if targetChannel == 0 {
		// search by ID
		if parsed, err := strconv.ParseInt(m.Content, 10, 64); err == nil {
			if gs.Channel(true, parsed) != nil {
				targetChannel = parsed
			}
		}
	}

	if targetChannel == 0 {
		s.sendMessage("Couldn't find that channel, say `this` or `here` for the current channel, otherwise type the name, id or mention it.")
		return
	}

	hasPerms, err := bot.AdminOrPermMS(targetChannel, dstate.MSFromDGoMember(gs, m.Member), discordgo.PermissionSendMessages)
	if err != nil {
		s.sendMessage("Failed retrieving your pems, check with bot owner")
		logger.WithError(err).WithField("guild", gs.ID).Error("failed calculating permissions")
		return
	}

	if !hasPerms {
		s.sendMessage("You don't have permissions to send messages there, please pick another channel")
		return
	}

	s.Channel = targetChannel
	s.State = SetupStateTitle

	s.sendMessage("Using channel <#%d>. Please enter a title for the event now!", s.Channel)
}

func (s *SetupSession) handleMessageSetupStateTitle(m *discordgo.Message) {
	if utf8.RuneCountInString(m.Content) > 256 {
		s.sendMessage("Title can only be 256 characters long! Enter a new title now.")
		return
	}

	s.Title = m.Content
	s.State = SetupStateMaxParticipants
	s.sendMessage("Set title of the event to **%s**, Enter the max number of people able to join (or 0 for no limit)", s.Title)
}

func (s *SetupSession) handleMessageSetupStateMaxParticipants(m *discordgo.Message) {
	participants, err := strconv.ParseInt(m.Content, 10, 32)
	if err != nil {
		s.sendMessage("That wasn't a number! Please enter a number.")
		return
	}

	s.MaxParticipants = int(participants)
	s.State = SetupStateWhen

	s.sendMessage("Set max participants to **%d**, now please enter when this event starts, in either your registered time zone (using the `setz` command) or UTC. (example: `tomorrow 10pm`, `10 may 2pm UTC`)", s.MaxParticipants)
}

var UTCRegex = regexp.MustCompile(`(?i)\butc\b`)

func (s *SetupSession) handleMessageSetupStateWhen(m *discordgo.Message) {
	registeredTimezone := timezonecompanion.GetUserTimezone(s.AuthorID)
	if registeredTimezone == nil || UTCRegex.MatchString(m.Content) {
		registeredTimezone = time.UTC
	}

	now := time.Now().In(registeredTimezone)
	t, err := dateParser.Parse(m.Content, now)
	// t, err := dateparse.ParseAny(m.Content)
	if err != nil || t == nil {
		s.sendMessage("Couldn't understand that date, Please try changing the format a little bit and try again\n||Error: %v||", err)
		return
	}

	s.When = t.Time
	s.State = SetupStateWhenConfirm

	in := common.HumanizeDuration(common.DurationPrecisionMinutes, t.Time.Sub(now))

	s.sendMessage("Set the starting time of the event to **%s** (in **%s**), is this correct? (`yes/no`)", t.Time.Format("02 Jan 2006 15:04 MST"), in)
}

func (s *SetupSession) handleMessageSetupStateWhenConfirm(m *discordgo.Message) {
	lower := strings.ToLower(m.Content)
	if len(lower) < 1 {
		return
	}

	if lower[0] == 'y' {
		s.Finish()
	} else {
		s.State = SetupStateWhen
		s.sendMessage("Please enter when this event starts. (example: `tomorrow 10pm`, `10 may 2pm`)")
	}
}

func (s *SetupSession) Finish() {

	// reserve the message
	reservedMessage, err := common.BotSession.ChannelMessageSendEmbed(s.Channel, &discordgo.MessageEmbed{Description: "Setting up RSVP Event..."})
	if err != nil {
		if code, _ := common.DiscordError(err); code != 0 {
			if code == discordgo.ErrCodeMissingPermissions || code == discordgo.ErrCodeMissingAccess {
				s.sendMessage("The bot doesn't have permissions to send embed messages there, check the permissions again...")
				go s.remove()
				return
			}
		}

		s.abortError("failed reserving message", err)
		return
	}

	localID, err := common.GenLocalIncrID(s.GuildID, "rsvp_session")
	if err != nil {
		s.abortError("failed generating local ID", err)
		return
	}

	// insert the model
	m := &models.RSVPSession{
		MessageID: reservedMessage.ID,

		AuthorID:  s.AuthorID,
		ChannelID: s.Channel,
		GuildID:   s.GuildID,
		LocalID:   localID,

		CreatedAt: time.Now(),
		StartsAt:  s.When,

		Title:           s.Title,
		MaxParticipants: s.MaxParticipants,
		SendReminders:   true,
	}

	err = m.InsertG(context.Background(), boil.Infer())
	if err != nil {
		s.abortError("failed inserting model", err)
		return
	}

	// set up the proper message
	err = UpdateEventEmbed(m)
	if err != nil {
		m.DeleteG(context.Background())
		s.abortError("failed updating the embed", err)
		return
	}

	err = AddReactions(m.ChannelID, m.MessageID)
	if err != nil {
		m.DeleteG(context.Background())
		s.abortError("failed adding reactions", err)
		return
	}

	err = scheduledevents2.ScheduleEvent("rsvp_update_session", m.GuildID, NextUpdateTime(m), m.MessageID)
	if err != nil {
		m.DeleteG(context.Background())
		s.abortError("failed scheduling update", err)
		return
	}

	go s.remove()

	// finish by deleting the setup messages
	toDelete := s.setupMessages
	if len(toDelete) > 100 {
		toDelete = toDelete[len(toDelete)-100:]
	}

	common.BotSession.ChannelMessagesBulkDelete(s.SetupChannel, toDelete)
}

func (s *SetupSession) abortError(msg string, err error) {
	logger.WithField("guild", s.GuildID).WithError(err).Error(msg)
	s.sendMessage("An error occurred, the setup has been canceled, please retry in a moment.")
	go s.remove()
}

func (s *SetupSession) loopCheckActive() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCH:
			return
		case <-ticker.C:
		}

		s.mu.Lock()
		since := time.Since(s.LastAction)

		if since < time.Minute*3 {
			s.mu.Unlock()
			continue
		}

		s.sendMessage("Event setup timed out, you waited over 3 minutes before doing anything, if you still want to set up a event then you have to restart from the beginning by issuing the `event create` commmand")
		s.mu.Unlock()

		// Session expired, remove it
		s.remove()
	}
}

func (s *SetupSession) remove() {
	s.mu.Lock()
	stopped := s.stopped
	s.stopped = true
	s.mu.Unlock()

	if stopped {
		return
	}

	close(s.stopCH)

	s.plugin.setupSessionsMU.Lock()
	defer s.plugin.setupSessionsMU.Unlock()

	for i, v := range s.plugin.setupSessions {
		if v == s {
			s.plugin.setupSessions = append(s.plugin.setupSessions[:i], s.plugin.setupSessions[i+1:]...)
		}
	}
}

func (s *SetupSession) sendMessage(msgf string, args ...interface{}) {
	m, err := common.BotSession.ChannelMessageSend(s.SetupChannel, "[RSVP Event Setup]: "+fmt.Sprintf(msgf, args...))
	if err != nil {
		logger.WithError(err).WithField("guild", s.GuildID).WithField("channel", s.SetupChannel).Error("failed sending setup message")
	} else {
		s.setupMessages = append(s.setupMessages, m.ID)
	}
}

const (
	EmojiJoining    = "✅"
	EmojiMaybe      = "❔"
	EmojiNotJoining = "❌"
	EmojiWaitlist   = "🕐"
)

var EventReactions = []string{EmojiJoining, EmojiNotJoining, EmojiWaitlist, EmojiMaybe}

func AddReactions(channelID, messageID int64) error {
	for _, r := range EventReactions {
		err := common.BotSession.MessageReactionAdd(channelID, messageID, r)
		if err != nil {
			return err
		}
	}

	return nil
}
