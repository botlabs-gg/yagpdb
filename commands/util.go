package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

type DurationArg struct {
	Min, Max time.Duration
}

func (d *DurationArg) Matches(def *dcmd.ArgDef, part string) bool {
	if len(part) < 1 {
		return false
	}

	// We "need" the first character to be a number
	r, _ := utf8.DecodeRuneInString(part)
	if !unicode.IsNumber(r) {
		return false
	}

	_, err := ParseDuration(part)
	return err == nil
}

func (d *DurationArg) Parse(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	dur, err := ParseDuration(part)
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

// Parses a time string like 1day3h
func ParseDuration(str string) (time.Duration, error) {
	var dur time.Duration

	currentNumBuf := ""
	currentModifierBuf := ""

	// Parse the time
	for _, v := range str {
		// Ignore whitespace
		if unicode.Is(unicode.White_Space, v) {
			continue
		}

		if unicode.IsNumber(v) {
			// If we reached a number and the modifier was also set, parse the last duration component before starting a new one
			if currentModifierBuf != "" {
				if currentNumBuf == "" {
					currentNumBuf = "1"
				}
				d, err := parseDurationComponent(currentNumBuf, currentModifierBuf)
				if err != nil {
					return d, err
				}

				dur += d

				currentNumBuf = ""
				currentModifierBuf = ""
			}

			currentNumBuf += string(v)

		} else {
			currentModifierBuf += string(v)
		}
	}

	if currentNumBuf != "" {
		d, err := parseDurationComponent(currentNumBuf, currentModifierBuf)
		if err != nil {
			return dur, errors.WrapIf(err, "not a duration")
		}

		dur += d
	}

	return dur, nil
}

func parseDurationComponent(numStr, modifierStr string) (time.Duration, error) {
	parsedNum, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	parsedDur := time.Duration(parsedNum)

	if strings.HasPrefix(modifierStr, "s") {
		parsedDur = parsedDur * time.Second
	} else if modifierStr == "" || (strings.HasPrefix(modifierStr, "m") && (len(modifierStr) < 2 || modifierStr[1] != 'o')) {
		parsedDur = parsedDur * time.Minute
	} else if strings.HasPrefix(modifierStr, "h") {
		parsedDur = parsedDur * time.Hour
	} else if strings.HasPrefix(modifierStr, "d") {
		parsedDur = parsedDur * time.Hour * 24
	} else if strings.HasPrefix(modifierStr, "w") {
		parsedDur = parsedDur * time.Hour * 24 * 7
	} else if strings.HasPrefix(modifierStr, "mo") {
		parsedDur = parsedDur * time.Hour * 24 * 30
	} else if strings.HasPrefix(modifierStr, "y") {
		parsedDur = parsedDur * time.Hour * 24 * 365
	} else {
		return parsedDur, errors.New("couldn't figure out what '" + numStr + modifierStr + "` was")
	}

	return parsedDur, nil

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
		if data.GS != nil {
			data.GS.RLock()
			cParentID := data.CS.ParentID
			data.GS.RUnlock()

			ms := data.MS

			channelOverrides, err := GetOverridesForChannel(data.CS.ID, cParentID, data.GS.ID)
			if err != nil {
				logger.WithError(err).WithField("guild", data.Msg.GuildID).Error("failed retrieving command overrides")
				return nil, nil
			}

			chain := []*dcmd.Container{CommandSystem.Root, container}

			enabled := false

			// make sure that at least 1 command in the container is enabled
			for _, v := range container.Commands {
				cast := v.Command.(*YAGCommand)
				settings, err := cast.GetSettingsWithLoadedOverrides(chain, data.GS.ID, channelOverrides)
				if err != nil {
					logger.WithError(err).WithField("guild", data.Msg.GuildID).Error("failed checking if command was enabled")
					continue
				}

				if len(settings.RequiredRoles) > 0 && !common.ContainsInt64SliceOneOf(settings.RequiredRoles, ms.Roles) {
					// missing required role
					continue
				}

				if len(settings.IgnoreRoles) > 0 && common.ContainsInt64SliceOneOf(settings.IgnoreRoles, ms.Roles) {
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

func (ma *MemberArg) Matches(def *dcmd.ArgDef, part string) bool {
	// Check for mention
	if strings.HasPrefix(part, "<@") && strings.HasSuffix(part, ">") {
		return true
	}

	// Check for ID
	_, err := strconv.ParseInt(part, 10, 64)
	if err == nil {
		return true
	}

	return false
}

func (ma *MemberArg) Parse(def *dcmd.ArgDef, part string, data *dcmd.Data) (interface{}, error) {
	id := ma.ExtractID(part, data)

	if id < 1 {
		return nil, dcmd.NewSimpleUserError("Invalid mention or id")
	}

	member, err := bot.GetMemberJoinedAt(data.GS.ID, id)
	if err != nil {
		if common.IsDiscordErr(err, discordgo.ErrCodeUnknownMember, discordgo.ErrCodeUnknownUser) {
			return nil, dcmd.NewSimpleUserError("User not a member of the server")
		}

		return nil, err
	}

	return member, nil
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
