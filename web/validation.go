package web

// Form validation tools

import (
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"regexp"
	"unicode/utf8"
)

type FormType int

const (
	FormTypeMessage = iota
	FormTypeMessageTemplate
	FormTypeChannel
	FormTypeRole
	FormTypeRegex
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
