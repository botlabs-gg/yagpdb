package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

type DurationArg struct {
	Min, Max time.Duration
}

var _ dcmd.ArgType = (*DurationArg)(nil)

func (d *DurationArg) CheckCompatibility(def *dcmd.ArgDef, part string) dcmd.CompatibilityResult {
	if len(part) < 1 {
		return dcmd.Incompatible
	}

	// We "need" the first character to be a number
	r, _ := utf8.DecodeRuneInString(part)
	if !unicode.IsNumber(r) {
		return dcmd.Incompatible
	}

	_, err := common.ParseDuration(part)
	if err != nil {
		return dcmd.Incompatible
	}
	return dcmd.CompatibilityGood
}

func (d *DurationArg) ParseFromMessage(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	dur, err := common.ParseDuration(part)
	if err != nil {
		return nil, err
	}

	if d.Min != 0 && d.Min > dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	if d.Max != 0 && d.Max < dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	return dur, nil
}

func (d *DurationArg) ParseFromInteraction(def *dcmd.ArgDef, data *dcmd.Data, options *dcmd.SlashCommandsParseOptions) (val interface{}, err error) {
	s, err := options.ExpectString(def.Name)
	if err != nil {
		return nil, err
	}
	dur, err := common.ParseDuration(s)
	if err != nil {
		return nil, err
	}

	if d.Min != 0 && d.Min > dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	if d.Max != 0 && d.Max < dur {
		return nil, &DurationOutOfRangeError{ArgName: def.Name, Got: dur, Max: d.Max, Min: d.Min}
	}

	return dur, nil
}

func (d *DurationArg) HelpName() string {
	return "Duration"
}

func (d *DurationArg) SlashCommandOptions(def *dcmd.ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionString)}
}

type DurationOutOfRangeError struct {
	Min, Max time.Duration
	Got      time.Duration
	ArgName  string
}

func (o *DurationOutOfRangeError) Error() string {
	preStr := "too big"
	if o.Got < o.Min {
		preStr = "too small"
	}

	if o.Min == 0 {
		return fmt.Sprintf("%s is %s, has to be smaller than %s", o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Max))
	} else if o.Max == 0 {
		return fmt.Sprintf("%s is %s, has to be bigger than %s", o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Min))
	} else {
		format := "%s is %s (has to be within `%s` and `%s`)"
		return fmt.Sprintf(format, o.ArgName, preStr, common.HumanizeDuration(common.DurationPrecisionMinutes, o.Min), common.HumanizeDuration(common.DurationPrecisionMinutes, o.Max))
	}
}

// PublicError is a error that is both logged and returned as a response
type PublicError string

func (p PublicError) Error() string {
	return string(p)
}

func NewPublicError(a ...interface{}) PublicError {
	return PublicError(fmt.Sprint(a...))
}

func NewPublicErrorF(f string, a ...interface{}) PublicError {
	return PublicError(fmt.Sprintf(f, a...))
}

// UserError is a special error type that is only sent as a response, and not logged
type UserError string

var _ dcmd.UserError = (UserError)("") // make sure it implements this interface

func (ue UserError) Error() string {
	return string(ue)
}

func (ue UserError) IsUserError() bool {
	return true
}

func NewUserError(a ...interface{}) error {
	return UserError(fmt.Sprint(a...))
}

func NewUserErrorf(f string, a ...interface{}) error {
	return UserError(fmt.Sprintf(f, a...))
}

func FilterBadInvites(msg string, guildID int64, replacement string) string {
	return common.ReplaceServerInvites(msg, guildID, replacement)
}

// CommonContainerNotFoundHandler is a common "NotFound" handler that should be used with dcmd containers
// it ensures that no messages is sent if none of the commands in te container is enabeld
// if "fixedMessage" is empty, then it shows default generated container help
func CommonContainerNotFoundHandler(container *dcmd.Container, fixedMessage string) func(data *dcmd.Data) (interface{}, error) {
	return func(data *dcmd.Data) (interface{}, error) {
		// Only show stuff if atleast 1 of the commands in the container is enabled
		if data.GuildData != nil {
			ms := data.GuildData.MS

			channelOverrides, err := GetOverridesForChannel(data.GuildData.CS, data.GuildData.GS)
			if err != nil {
				logger.WithError(err).WithField("guild", data.GuildData.GS.ID).Error("failed retrieving command overrides")
				return nil, nil
			}

			chain := []*dcmd.Container{CommandSystem.Root, container}

			enabled := false

			// make sure that at least 1 command in the container is enabled
			for _, v := range container.Commands {
				cast := v.Command.(*YAGCommand)
				settings, err := cast.GetSettingsWithLoadedOverrides(chain, data.GuildData.GS.ID, channelOverrides)
				if err != nil {
					logger.WithError(err).WithField("guild", data.GuildData.GS.ID).Error("failed checking if command was enabled")
					continue
				}

				if len(settings.RequiredRoles) > 0 && !common.ContainsInt64SliceOneOf(settings.RequiredRoles, ms.Member.Roles) {
					// missing required role
					continue
				}

				if len(settings.IgnoreRoles) > 0 && common.ContainsInt64SliceOneOf(settings.IgnoreRoles, ms.Member.Roles) {
					// has ignored role
					continue
				}

				if settings.Enabled {
					enabled = true
					break
				}
			}

			// no commands enabled, do nothing
			if !enabled {
				return nil, nil
			}
		}

		if fixedMessage != "" {
			return fixedMessage, nil
		}

		resp := dcmd.GenerateHelp(data, container, &dcmd.StdHelpFormatter{})
		if len(resp) > 0 {
			return resp[0], nil
		}

		return nil, nil
	}
}

// MemberArg matches a id or mention and returns a MemberState object for the user
type MemberArg struct{}

var _ dcmd.ArgType = (*MemberArg)(nil)

func (ma *MemberArg) CheckCompatibility(def *dcmd.ArgDef, part string) dcmd.CompatibilityResult {
	// Check for mention
	if strings.HasPrefix(part, "<@") && strings.HasSuffix(part, ">") {
		return dcmd.DetermineSnowflakeCompatibility(strings.TrimPrefix(part[2:len(part)-1], "!"))
	}

	// Check for ID
	return dcmd.DetermineSnowflakeCompatibility(part)
}

func (ma *MemberArg) ParseFromMessage(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	id := ma.ExtractID(part, data)

	if id < 1 {
		return nil, dcmd.NewSimpleUserError("Invalid mention or id")
	}

	member, err := bot.GetMember(data.GuildData.GS.ID, id)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember, discordgo.ErrCodeUnknownUser) {
			return nil, dcmd.NewSimpleUserError("User not a member of the server")
		}

		return nil, err
	}

	return member, nil
}

func (ma *MemberArg) ParseFromInteraction(def *dcmd.ArgDef, data *dcmd.Data, options *dcmd.SlashCommandsParseOptions) (val interface{}, err error) {
	member, err := options.ExpectMember(def.Name)
	if err != nil {
		return nil, err
	}

	return dstate.MemberStateFromMember(member), nil
}

func (ma *MemberArg) ExtractID(part string, data *dcmd.Data) int64 {
	if strings.HasPrefix(part, "<@") && len(part) > 3 {
		// Direct mention
		id := part[2 : len(part)-1]
		if id[0] == '!' {
			// Nickname mention
			id = id[1:]
		}

		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return -1
		}

		return parsed
	}

	id, err := strconv.ParseInt(part, 10, 64)
	if err == nil {
		return id
	}

	return -1
}

func (ma *MemberArg) HelpName() string {
	return "Member"
}

func (ma *MemberArg) SlashCommandOptions(def *dcmd.ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionUser)}
}

type EphemeralOrGuild struct {
	Content string
	Embed   *discordgo.MessageEmbed
}

var _ dcmd.Response = (*EphemeralOrGuild)(nil)

func (e *EphemeralOrGuild) Send(data *dcmd.Data) ([]*discordgo.Message, error) {

	switch data.TriggerType {
	case dcmd.TriggerTypeSlashCommands:
		tmp := &EphemeralOrNone{
			Content: e.Content,
			Embed:   e.Embed,
		}
		return tmp.Send(data)
	default:
		send := &discordgo.MessageSend{
			Content:         e.Content,
			Embeds:          []*discordgo.MessageEmbed{e.Embed},
			AllowedMentions: discordgo.AllowedMentions{},
		}

		return data.SendFollowupMessage(send, discordgo.AllowedMentions{})
	}
}

type EphemeralOrNone struct {
	Content string
	Embed   *discordgo.MessageEmbed
}

var _ dcmd.Response = (*EphemeralOrNone)(nil)

func (e *EphemeralOrNone) Send(data *dcmd.Data) ([]*discordgo.Message, error) {

	switch data.TriggerType {
	case dcmd.TriggerTypeSlashCommands:
		params := &discordgo.WebhookParams{
			Content:         e.Content,
			AllowedMentions: &discordgo.AllowedMentions{},
			Flags:           64,
		}

		if e.Embed != nil {
			params.Embeds = []*discordgo.MessageEmbed{e.Embed}
		}

		// _, err := data.Session.EditOriginalInteractionResponse(common.BotApplication.ID, data.SlashCommandTriggerData.Interaction.Token, &discordgo.EditWebhookMessageRequest{
		// 	Content: "Failed running the command.",
		// })

		if yc, ok := data.Cmd.Command.(*YAGCommand); ok {
			settings, err := yc.GetSettings(data.ContainerChain, data.GuildData.CS, data.GuildData.GS)
			if err != nil {
				return nil, err
			}
			if !(yc.IsResponseEphemeral || settings.AlwaysEphemeral) {
				// Yeah so because the original reaction response is not marked as ephemeral, and there's no way to change that, just delete it i guess...
				// because otherwise the followup message turns into the original response
				err := data.Session.DeleteInteractionResponse(common.BotApplication.ID, data.SlashCommandTriggerData.Interaction.Token)
				if err != nil {
					return nil, err
				}
			}
		}

		m, err := data.Session.CreateFollowupMessage(common.BotApplication.ID, data.SlashCommandTriggerData.Interaction.Token, params)
		// m, err := data.Session.EditOriginalInteractionResponse(common.BotApplication.ID, data.SlashCommandTriggerData.Interaction.Token, params)
		// err = data.Session.CreateInteractionResponse(data.SlashCommandTriggerData.Interaction.ID, data.SlashCommandTriggerData.Interaction.Token, &discordgo.InteractionResponse{
		// 	Kind: discordgo.InteractionResponseTypeChannelMessageWithSource,
		// 	Data: &discordgo.InteractionApplicationCommandCallbackData{
		// 		Content: &e.Content,
		// 		Flags:   64,
		// 	},
		// })
		if err != nil {
			return nil, err
		}

		// return []*discordgo.Message{}, nil
		return []*discordgo.Message{m}, nil
	default:
		return nil, nil
	}
}

// RoleArg matches an id or name and returns a discordgo.Role
type RoleArg struct{}

var _ dcmd.ArgType = (*RoleArg)(nil)

func (ra *RoleArg) CheckCompatibility(def *dcmd.ArgDef, part string) dcmd.CompatibilityResult {
	// Check for mention
	if strings.HasPrefix(part, "<@&") && strings.HasSuffix(part, ">") {
		return dcmd.DetermineSnowflakeCompatibility(part[3 : len(part)-1])
	}

	if part != "" {
		// role name can be essentially any string
		return dcmd.CompatibilityGood
	}
	return dcmd.Incompatible
}

func (ra *RoleArg) ParseFromMessage(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	id := ra.ExtractID(part, data)

	var idName string
	switch t := id.(type) {
	case int, int32, int64:
		idName = strconv.FormatInt(t.(int64), 10)
	case string:
		idName = t
	default:
		idName = ""
	}

	for _, v := range data.GuildData.GS.Roles {
		if v.ID == id {
			return &v, nil
		} else if v.Name == idName {
			return &v, nil
		}
	}

	return nil, dcmd.NewSimpleUserError("Invalid role mention or id")
}

func (ra *RoleArg) ParseFromInteraction(def *dcmd.ArgDef, data *dcmd.Data, options *dcmd.SlashCommandsParseOptions) (val interface{}, err error) {
	r, err := options.ExpectRole(def.Name)
	return r, err
}

func (ra *RoleArg) SlashCommandOptions(def *dcmd.ArgDef) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{def.StandardSlashCommandOption(discordgo.ApplicationCommandOptionRole)}
}

func (ra *RoleArg) ExtractID(part string, data *dcmd.Data) interface{} {
	if strings.HasPrefix(part, "<@&") && len(part) > 3 {
		// Direct mention
		id := part[3 : len(part)-1]

		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return -1
		}

		return parsed
	}

	id, err := strconv.ParseInt(part, 10, 64)
	if err == nil {
		return id
	}

	return part
}

func (ra *RoleArg) HelpName() string {
	return "Role"
}
