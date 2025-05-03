package dcmd

import (
	"context"
	"reflect"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/pkg/errors"
)

// Data is  a struct of data available to commands
type Data struct {
	Cmd      *RegisteredCommand
	Args     []*ParsedArg
	Switches map[string]*ParsedArg

	// These fields are always available
	ChannelID int64
	Author    *discordgo.User

	// Only set if this command was not ran through slash commands
	TraditionalTriggerData *TraditionalTriggerData

	// Only set if this command was ran through discord slash commands
	SlashCommandTriggerData *SlashCommandTriggerData

	// Only provided if the command was ran in a DM Context
	GuildData *GuildContextData

	// The session that triggered the command
	Session *discordgo.Session

	Source      TriggerSource
	TriggerType TriggerType

	// The chain of containers we went through, first element is always root
	ContainerChain []*Container

	// The system that triggered this command
	System *System

	context context.Context
}

type GuildContextData struct {
	CS *dstate.ChannelState
	GS *dstate.GuildSet
	MS *dstate.MemberState
}

type SlashCommandTriggerData struct {
	Interaction *discordgo.Interaction
	// The options slice for the command options themselves
	// This is a helper so you don't have to dig it out yourself in the case of nested subcommands
	Options []*discordgo.ApplicationCommandInteractionDataOption
}

type TraditionalTriggerData struct {
	Message               *discordgo.Message
	MessageStrippedPrefix string
	PrefixUsed            string
}

// Context returns an always non-nil context
func (d *Data) Context() context.Context {
	if d.context == nil {
		return context.Background()
	}

	return d.context
}

func (d *Data) Switch(name string) *ParsedArg {
	return d.Switches[name]
}

// WithContext creates a copy of d with the context set to ctx
func (d *Data) WithContext(ctx context.Context) *Data {
	cop := new(Data)
	*cop = *d
	cop.context = ctx
	return cop
}

func (d *Data) SendFollowupMessage(reply interface{}, allowedMentions discordgo.AllowedMentions) ([]*discordgo.Message, error) {
	switch t := reply.(type) {
	case Response:
		return t.Send(d)
	case ManualResponse:
		return t.Messages, nil
	case string:
		if t != "" {
			return SplitSendMessage(d, t, allowedMentions)
		}
		return []*discordgo.Message{}, nil
	case error:
		if t != nil {
			m := t.Error()
			return SplitSendMessage(d, m, allowedMentions)
		}
		return []*discordgo.Message{}, nil
	case *discordgo.MessageEmbed:

		switch d.TriggerType {
		case TriggerTypeSlashCommands:
			m, err := d.Session.CreateFollowupMessage(d.SlashCommandTriggerData.Interaction.ApplicationID, d.SlashCommandTriggerData.Interaction.Token, &discordgo.WebhookParams{
				Embeds:          []*discordgo.MessageEmbed{t},
				AllowedMentions: &allowedMentions,
			})
			return []*discordgo.Message{m}, err
		default:
			m, err := d.Session.ChannelMessageSendEmbed(d.ChannelID, t)
			return []*discordgo.Message{m}, err
		}
	case []*discordgo.MessageEmbed:
		msgs := make([]*discordgo.Message, 0, len(t))
		switch d.TriggerType {
		case TriggerTypeSlashCommands:
			cur := 0
			for {
				next := t[cur:]
				if len(next) > 10 {
					next = next[:10]
				}

				params := &discordgo.WebhookParams{
					Embeds:          next,
					AllowedMentions: &allowedMentions,
				}

				m, err := d.Session.CreateFollowupMessage(d.SlashCommandTriggerData.Interaction.ApplicationID, d.SlashCommandTriggerData.Interaction.Token, params)
				if err != nil {
					return msgs, err
				}

				msgs = append(msgs, m)

				if len(t[cur:]) <= 10 {
					break
				}

				cur += 10
			}
		default:
			for _, embed := range t {
				m, err := d.Session.ChannelMessageSendEmbed(d.ChannelID, embed)
				if err != nil {
					return msgs, err
				}
				msgs = append(msgs, m)
			}
		}

		return msgs, nil

	case *discordgo.MessageSend:
		switch d.TriggerType {
		case TriggerTypeSlashCommands:
			params := &discordgo.WebhookParams{
				Content:         t.Content,
				TTS:             t.TTS,
				AllowedMentions: &t.AllowedMentions,
				File:            t.File,
				Components:      t.Components,
				Flags:           int64(t.Flags),
			}

			if len(t.Files) > 0 {
				params.File = t.Files[0]
			}

			if len(t.Embeds) > 0 {
				params.Embeds = t.Embeds
			}

			m, err := d.Session.CreateFollowupMessage(d.SlashCommandTriggerData.Interaction.ApplicationID, d.SlashCommandTriggerData.Interaction.Token, params)
			return []*discordgo.Message{m}, err

		default:
			m, err := d.Session.ChannelMessageSendComplex(d.ChannelID, t)
			return []*discordgo.Message{m}, err
		}
	case []*discordgo.ApplicationCommandOptionChoice:
		if d.TriggerType == TriggerTypeSlashCommands {
			if d.SlashCommandTriggerData.Interaction.Type != discordgo.InteractionApplicationCommandAutocomplete {
				return nil, errors.New("Cannot use autocomplete with interaction type: " + d.SlashCommandTriggerData.Interaction.Type.String())
			}
			err := d.Session.CreateInteractionResponse(d.SlashCommandTriggerData.Interaction.ID, d.SlashCommandTriggerData.Interaction.Token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Choices: t,
				},
			})
			return nil, err
		}
	}

	return nil, errors.New("Unknown reply type: " + reflect.TypeOf(reply).String() + " (Does not implement Response)")
}

// Where this command comes from
type TriggerSource int

const (
	TriggerSourceGuild TriggerSource = iota
	TriggerSourceDM
)

type TriggerType int

const (
	// triggered directly somehow, no prefix
	TriggerTypeDirect TriggerType = iota

	// triggered through a mention trigger
	TriggerTypeMention

	// triggered through a prefix trigger
	TriggerTypePrefix

	// triggered through slash commands or autocomplete
	TriggerTypeSlashCommands
)
