package web

// Form validation tools

import (
	"errors"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"strconv"
	"unicode/utf8"
)

type FormType int

const (
	FormTypeMessage = iota
	FormTypeMessageTemplate
	FormTypeChannel
	FormTypeRole
	FormTypeRegex
	FormTypeInt
	FormTypeFloat
)

var (
	ErrTooLong         = errors.New("Too Long")
	ErrChannelNotFound = errors.New("Channel not found")
	ErrRoleNotFound    = errors.New("Role not found")
)

// A formfield to validate
type FormField struct {
	Type  FormType
	Value string
	Name  string
	Max   int
	Min   int

	MaxF float64
	MinF float64
}

// Returns true if somethign was wrong
func (f *FormField) Validate(guild *discordgo.Guild, tmpl TemplateData) bool {
	max := 2000
	if f.Max != 0 {
		max = f.Max
	}

	var err error
	switch f.Type {
	case FormTypeMessage:
		err = ValidateMessageForm(f.Value, max)
	case FormTypeMessageTemplate:
		err = ValidateMessageTemplateForm(f.Value, max)
	case FormTypeChannel:
		err = ValidateChannelForm(f.Value, guild.Channels)
	case FormTypeRegex:
		err = ValidateRegexForm(f.Value)
	case FormTypeRole:
		err = ValidateRoleForm(f.Value, guild.Roles)
	case FormTypeInt:
		err = ValidateIntForm(f.Value, f.Min, f.Max)
	case FormTypeFloat:
		err = ValidateFloatForm(f.Value, f.MinF, f.MaxF)
	}

	if err == nil {
		return true
	}

	tmpl.AddAlerts(ErrorAlert(f.Name + " field: " + err.Error()))
	return false
}

func ValidateForm(guild *discordgo.Guild, tmpl TemplateData, fields []*FormField) bool {
	ok := true
	for _, field := range fields {
		newOk := field.Validate(guild, tmpl)
		if !newOk {
			ok = false
		}
	}

	if !ok {
		tmpl.AddAlerts(ErrorAlert("Form is invalid, please fix the above and try again (contact me on discord if you're still having issues, server is above)"))
	}
	return ok
}

func ValidateIntForm(s string, min, max int) error {
	parsed, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	if min != max && (parsed < min || parsed > max) {
		return fmt.Errorf("Out of range (%d - %d", min, max)
	}

	return nil
}

func ValidateFloatForm(s string, min, max float64) error {
	parsed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}

	if min != max && (parsed < min || parsed > max) {
		return fmt.Errorf("Out of range (%d - %d", min, max)
	}

	return nil
}

func ValidateRegexForm(s string) error {
	_, err := regexp.Compile(s)
	return err
}

func ValidateMessageForm(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}
	return nil
}

func ValidateMessageTemplateForm(s string, max int) error {
	if err := ValidateMessageForm(s, max); err != nil {
		return err
	}

	_, err := common.ParseExecuteTemplate(s, nil)
	return err
}

func ValidateChannelForm(s string, channels []*discordgo.Channel) error {
	if s == "" {
		return nil
	}

	for _, v := range channels {
		if s == v.ID {
			return nil
		}
	}

	return ErrChannelNotFound
}

func ValidateRoleForm(s string, roles []*discordgo.Role) error {
	if s == "" {
		return nil
	}

	for _, v := range roles {
		if s == v.ID {
			return nil
		}
	}

	return ErrRoleNotFound
}
