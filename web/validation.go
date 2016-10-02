package web

// Form validation tools
// Pass it a struct and it will validate each field
// depending on struct tags
//
// float/int: `valid:"{min],{max}"`
//    - Makes sure the float/int is whitin min and max
// normal string: `valid:",{maxLen}"`
//    - Makes sure the string is shorter than maxLen)
// regex string: `valid:"regex,{maxLen}"`
//    - Makes sure the string is shorter than maxLen)
//    - Makes sure the regex compiles without errors
// template string: `valid:"tmpl,{maxLen}"`
//    - Makes sure the string is shorter than maxLen)
//    - Makes sure the templates parses without errors
// channel string:  `valid:"channel,{allowEmpty}"`
//    - Makes sure the channel is part of the guild
// role string:  `valid:"role,{allowEmpty}"`
//    - Makes sure the role is part of the guild

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrTooLong         = errors.New("Too Long")
	ErrChannelNotFound = errors.New("Channel not found")
	ErrRoleNotFound    = errors.New("Role not found")
)

// Probably needs some cleaning up
func ValidateForm(guild *discordgo.Guild, tmpl TemplateData, form interface{}) bool {

	ok := true

	v := reflect.Indirect(reflect.ValueOf(form))
	t := v.Type()

	numFields := v.NumField()
	for i := 0; i < numFields; i++ {
		tField := t.Field(i)
		tag := tField.Tag
		validation := tag.Get("valid")
		if validation == "" {
			continue
		}

		vField := v.Field(i)

		validationSplit := strings.Split(validation, ",")

		var err error

		switch cv := vField.Interface().(type) {
		case int:
			min, max := readMinMax(validationSplit)
			err = ValidateIntField(int64(cv), int64(min), int64(max))
		case float64:
			min, max := readMinMax(validationSplit)
			err = ValidateFloatField(cv, min, max)
		case float32:
			min, max := readMinMax(validationSplit)
			err = ValidateFloatField(float64(cv), min, max)
		case string:
			if len(validationSplit) < 1 {
				continue
			}
			maxLen := 2000

			// Retrieve max len from tag is needed
			if len(validationSplit) > 1 && (validationSplit[0] == "tmpl" || validationSplit[0] == "regex" || validationSplit[0] == "") {
				newMaxLen, err := strconv.ParseInt(validationSplit[1], 10, 32)
				if err != nil {
					logrus.WithError(err).Error("Failed parsing int")
				} else {
					maxLen = int(newMaxLen)
				}
			}

			// Treat non empty as true
			allowEmpty := false
			if len(validationSplit) > 1 {
				if validationSplit[1] != "false" {
					allowEmpty = true
				}
			}

			switch validationSplit[0] {
			case "tmpl":
				err = ValidateTemplateField(cv, maxLen)
			case "regex":
				err = ValidateRegexField(cv, maxLen)
			case "role":
				err = ValidateRoleField(cv, guild.Roles, allowEmpty)
			case "channel":
				err = ValidateChannelField(cv, guild.Channels, allowEmpty)
			default:
				err = ValidateStringField(cv, maxLen)
			}
		}

		if err != nil {

			// Create a pretty name for the field by turing: "AnnounceMessage" into "Announce Message"
			prettyField := ""
			for _, r := range tField.Name {
				if unicode.IsUpper(r) {
					if prettyField != "" {
						prettyField += " "
					}
				}

				prettyField += string(r)
			}
			prettyField = strings.TrimSpace(prettyField)

			tmpl.AddAlerts(ErrorAlert(prettyField, ": ", err.Error()))
			ok = false
		}
	}

	if !ok {
		tmpl.AddAlerts(ErrorAlert("Form is invalid, please fix the above and try again (contact me on discord if you're still having issues, server link is above)"))
	}

	return ok
}

func readMinMax(split []string) (float64, float64) {
	if len(split) < 1 {
		return 0, 0
	}

	min, _ := strconv.ParseFloat(split[0], 64)

	max := float64(0)
	if len(split) > 1 {
		max, _ = strconv.ParseFloat(split[1], 64)
	}
	return min, max
}

func ValidateIntField(i int64, min, max int64) error {

	if min != max && (i < min || i > max) {
		return fmt.Errorf("Out of range (%d - %d)", min, max)
	}

	return nil
}

func ValidateFloatField(f float64, min, max float64) error {

	if min != max && (f < min || f > max) {
		return fmt.Errorf("Out of range (%f - %f)", min, max)
	}

	return nil
}

func ValidateRegexField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}

	_, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	return nil
}

func ValidateStringField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}
	return nil
}

func ValidateTemplateField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}

	_, err := common.ParseExecuteTemplate(s, nil)
	return err
}

func ValidateChannelField(s string, channels []*discordgo.Channel, allowEmpty bool) error {
	if s == "" {
		if allowEmpty {
			return nil
		} else {
			return errors.New("No channel specified")
		}
	}

	for _, v := range channels {
		if s == v.ID {
			return nil
		}
	}

	return ErrChannelNotFound
}

func ValidateRoleField(s string, roles []*discordgo.Role, allowEmpty bool) error {
	if s == "" {
		if allowEmpty {
			return nil
		} else {
			return errors.New("No role specified")
		}
	}

	for _, v := range roles {
		if s == v.ID {
			return nil
		}
	}

	return ErrRoleNotFound
}
