package dcmd

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/pkg/errors"
)

type System struct {
	Root           *Container
	Prefix         PrefixProvider
	ResponseSender ResponseSender
	State          dstate.StateTracker
}

func NewStandardSystem(staticPrefix string) (system *System) {
	sys := &System{
		Root:           &Container{HelpTitleEmoji: "ℹ️", HelpColor: 0xbeff7a},
		ResponseSender: &StdResponseSender{LogErrors: true},
	}
	if staticPrefix != "" {
		sys.Prefix = NewSimplePrefixProvider(staticPrefix)
	}

	sys.Root.AddMidlewares(ArgParserMW)

	return sys
}

// You can add this as a handler directly to discordgo, it will recover from any panics that occured in commands
// and log errors using the standard logger
func (sys *System) HandleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Set up handler to recover from panics
	defer func() {
		if r := recover(); r != nil {
			sys.handlePanic(s, r, false)
		}
	}()

	err := sys.CheckMessage(s, m)
	if err != nil {
		log.Println("[DCMD ERROR]: Failed checking message:", err)
	}
}

// CheckMessage checks the message for commands, and triggers any command that the message should trigger
// you should not add this as an discord handler directly, if you want to do that you should add "system.HandleMessageCreate" instead.
func (sys *System) CheckMessage(s *discordgo.Session, m *discordgo.MessageCreate) error {

	data, err := sys.FillDataLegacyMessage(s, m.Message)
	if err != nil {
		return err
	}

	if !sys.FindPrefix(data) {
		// No prefix found in the message for a command to be triggered
		return nil
	}

	response, err := sys.Root.Run(data)
	return sys.ResponseSender.SendResponse(data, response, err)
}

// CheckInteraction checks a interaction and runs a command if found
func (sys *System) CheckInteraction(s *discordgo.Session, interaction *discordgo.Interaction) error {

	data, err := sys.FillDataInteraction(s, interaction)
	if err != nil {
		return err
	}

	response, err := sys.Root.Run(data)
	return sys.ResponseSender.SendResponse(data, response, err)
}

// CheckMessageWtihPrefetchedPrefix is the same as CheckMessage but you pass in a prefetched command prefix
func (sys *System) CheckMessageWtihPrefetchedPrefix(s *discordgo.Session, m *discordgo.MessageCreate, prefetchedPrefix string) error {

	data, err := sys.FillDataLegacyMessage(s, m.Message)
	if err != nil {
		return err
	}

	if !sys.FindPrefixWithPrefetched(data, prefetchedPrefix) {
		// No prefix found in the message for a command to be triggered
		return nil
	}

	response, err := sys.Root.Run(data)
	return sys.ResponseSender.SendResponse(data, response, err)
}

// FindPrefix checks if the message has a proper command prefix (either from the PrefixProvider or a direction mention to the bot)
// It sets the source field, and MsgStripped in data if found
func (sys *System) FindPrefix(data *Data) (found bool) {
	if data.Source == TriggerSourceDM {
		data.TraditionalTriggerData.MessageStrippedPrefix = data.TraditionalTriggerData.Message.Content
		data.TriggerType = TriggerTypeDirect
		return true
	}

	if sys.FindMentionPrefix(data) {
		return true
	}

	// Check for custom prefix
	if sys.Prefix == nil {
		return false
	}

	prefix := sys.Prefix.Prefix(data)
	if prefix == "" {
		return false
	}

	data.TraditionalTriggerData.PrefixUsed = prefix

	if strings.HasPrefix(data.TraditionalTriggerData.Message.Content, prefix) {
		data.TriggerType = TriggerTypePrefix
		data.TraditionalTriggerData.MessageStrippedPrefix = strings.TrimSpace(strings.Replace(data.TraditionalTriggerData.Message.Content, prefix, "", 1))
		found = true
	}

	return
}

// FindPrefixWithPrefetched is the same as FindPrefix but you pass in a prefetched command prefix
func (sys *System) FindPrefixWithPrefetched(data *Data, commandPrefix string) (found bool) {
	msg := data.TraditionalTriggerData.Message
	if data.Source == TriggerSourceDM {
		data.TraditionalTriggerData.MessageStrippedPrefix = msg.Content
		data.TriggerType = TriggerTypeDirect
		return true
	}

	if sys.FindMentionPrefix(data) {
		return true
	}

	data.TraditionalTriggerData.PrefixUsed = commandPrefix

	if strings.HasPrefix(msg.Content, commandPrefix) {
		data.TriggerType = TriggerTypePrefix
		data.TraditionalTriggerData.MessageStrippedPrefix = strings.TrimSpace(strings.Replace(msg.Content, commandPrefix, "", 1))
		found = true
	}

	return
}

func (sys *System) FindMentionPrefix(data *Data) (found bool) {
	if data.Session.State.User == nil {
		return false
	}

	ok := false
	stripped := ""

	msg := data.TraditionalTriggerData.Message

	// Check for mention
	id := discordgo.StrID(data.Session.State.User.ID)
	if strings.Index(msg.Content, "<@"+id+">") == 0 { // Normal mention
		ok = true
		stripped = strings.Replace(msg.Content, "<@"+id+">", "", 1)
		data.TraditionalTriggerData.PrefixUsed = "<@" + id + ">"
	} else if strings.Index(msg.Content, "<@!"+id+">") == 0 { // Nickname mention
		ok = true
		data.TraditionalTriggerData.PrefixUsed = "<@!" + id + ">"
		stripped = strings.Replace(msg.Content, "<@!"+id+">", "", 1)
	}

	if ok {
		data.TraditionalTriggerData.MessageStrippedPrefix = strings.TrimSpace(stripped)
		data.TriggerType = TriggerTypeMention
		return true
	}

	return false

}

var (
	ErrChannelNotFound               = errors.New("Channel not found")
	ErrGuildNotFound                 = errors.New("Guild not found")
	ErrMemberNotAvailable            = errors.New("Member not provided in message")
	ErrMemberNotAvailableInteraction = errors.New("Member not provided in interaction")
)

func (sys *System) FillDataLegacyMessage(s *discordgo.Session, m *discordgo.Message) (*Data, error) {
	var gs *dstate.GuildSet
	var cs *dstate.ChannelState

	if m.GuildID != 0 {
		gs = sys.State.GetGuild(m.GuildID)
		if gs == nil {
			return nil, ErrGuildNotFound
		}

		cs = gs.GetChannelOrThread(m.ChannelID)
		if cs == nil {
			return nil, ErrChannelNotFound
		}
	}

	data := &Data{
		Session:   s,
		System:    sys,
		ChannelID: m.ChannelID,
		Author:    m.Author,
		TraditionalTriggerData: &TraditionalTriggerData{
			Message: m,
		},
	}

	if m.GuildID == 0 {
		data.Source = TriggerSourceDM
	} else {
		data.Source = TriggerSourceGuild

		if m.Member == nil || m.Author == nil {
			return nil, ErrMemberNotAvailable
		}

		member := *m.Member
		member.User = m.Author // user field is not provided in Message.Member, its weird but *shrug*
		member.GuildID = m.GuildID

		data.GuildData = &GuildContextData{
			CS: cs,
			GS: gs,
			MS: dstate.MemberStateFromMember(&member),
		}
	}

	return data, nil
}

func (sys *System) FillDataInteraction(s *discordgo.Session, interaction *discordgo.Interaction) (*Data, error) {

	var gs *dstate.GuildSet
	var cs *dstate.ChannelState

	if interaction.GuildID != 0 {
		gs = sys.State.GetGuild(interaction.GuildID)
		if gs == nil {
			return nil, ErrGuildNotFound
		}

		cs = gs.GetChannelOrThread(interaction.ChannelID)
		if cs == nil {
			return nil, ErrChannelNotFound
		}
	}

	user := interaction.User
	if interaction.Member != nil {
		user = interaction.Member.User
	}

	data := &Data{
		Session:     s,
		System:      sys,
		ChannelID:   interaction.ChannelID,
		Author:      user,
		TriggerType: TriggerTypeSlashCommands,
		SlashCommandTriggerData: &SlashCommandTriggerData{
			Interaction: interaction,
		},
	}

	if interaction.GuildID == 0 {
		data.Source = TriggerSourceDM
	} else {
		data.Source = TriggerSourceGuild

		// were working off the assumption that member is always provided when in a guild
		if interaction.Member == nil {
			return nil, ErrMemberNotAvailableInteraction
		}

		member := *interaction.Member
		member.GuildID = interaction.GuildID

		data.GuildData = &GuildContextData{
			CS: cs,
			GS: gs,
			MS: dstate.MemberStateFromMember(&member),
		}
	}

	return data, nil
}

func (sys *System) handlePanic(s *discordgo.Session, r interface{}, sendChatNotice bool) {
	// TODO
	stack := debug.Stack()
	log.Printf("[DCMD PANIC] %v\n%s", r, string(stack))
}

// Retrieves the prefix that might be different on a per server basis
type PrefixProvider interface {
	Prefix(data *Data) string
}

// Simple Prefix provider for global fixed prefixes
type SimplePrefixProvider struct {
	prefix string
}

func NewSimplePrefixProvider(prefix string) PrefixProvider {
	return &SimplePrefixProvider{prefix: prefix}
}

func (pp *SimplePrefixProvider) Prefix(d *Data) string {
	return pp.prefix
}

func Indent(depth int) string {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "-"
	}
	return indent
}

type ResponseSender interface {
	SendResponse(cmdData *Data, resp interface{}, err error) error
}

type StdResponseSender struct {
	LogErrors bool
}

func (s *StdResponseSender) SendResponse(cmdData *Data, resp interface{}, err error) error {
	if err != nil && s.LogErrors {
		log.Printf("[DCMD]: Command %q returned an error: %s", cmdData.Cmd.FormatNames(false, "/"), err)
	}

	var errR error
	if resp == nil && err != nil {
		_, errR = SendResponseInterface(cmdData, fmt.Sprintf("%q command returned an error: %s", cmdData.Cmd.FormatNames(false, "/"), err), true)
	} else if resp != nil {
		_, errR = SendResponseInterface(cmdData, resp, false)
	}

	return errR
}
