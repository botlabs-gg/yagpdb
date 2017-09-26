package web

// Form validation tools
// Pass it a struct and it will validate each field
// depending on struct tags
//
// struct: `valid:"traverse"`
//	  - Validates the struct
// float/int: `valid:"{min],{max}"` or (for int64's) `valid:"role/channel,{allowEmpty}}"
//    - Makes sure the float/int is whitin min and max
// normal string: `valid:",{minLen},{maxLen},opts..."` or `valid:",{maxLen},opts..."`
//    - Makes sure the string is shorter than maxLen and bigger than minLen
// regex string: `valid:"regex,{maxLen}"`
//    - Makes sure the string is shorter than maxLen
//    - Makes sure the regex compiles without errors
// template string: `valid:"tmpl,{maxLen}"`
//    - Makes sure the string is shorter than maxLen)
//    - Makes sure the templates parses without errors
// channel string:  `valid:"channel,{allowEmpty}"`
//    - Makes sure the channel is part of the guild
// role string:  `valid:"role,{allowEmpty}"`
//    - Makes sure the role is part of the guild
//
// []int64 and []string applies the validation tags on the individual elements
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

type ValidationTag struct {
	values []string
}

func ParseValidationTag(tag string) *ValidationTag {
	fields := strings.Split(tag, ",")
	return &ValidationTag{
		values: fields,
	}
}

func (p *ValidationTag) Str(index int) (string, bool) {
	if len(p.values) <= index {
		return "", false
	}
	return p.values[index], true
}

func (p *ValidationTag) Float(index int) (float64, bool) {
	s, ok := p.Str(index)
	if !ok {
		return 0, false
	}

	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func (p *ValidationTag) Int(index int) (int, bool) {
	f, ok := p.Float(index)
	return int(f), ok
}

func (p *ValidationTag) Len() int {
	return len(p.values)
}

var (
	ErrChannelNotFound = errors.New("Channel not found")
	ErrRoleNotFound    = errors.New("Role not found")
)

// Probably needs some cleaning up
func ValidateForm(guild *discordgo.Guild, tmpl TemplateData, form interface{}) bool {

	ok := true

	v := reflect.Indirect(reflect.ValueOf(form))
	t := v.Type()

	// Walk over each field and look for valid tag
	numFields := v.NumField()
	for i := 0; i < numFields; i++ {
		tField := t.Field(i)
		tag := tField.Tag
		validationStr := tag.Get("valid")
		if validationStr == "" {
			continue
		}

		validationTag := ParseValidationTag(validationStr)
		vField := v.Field(i)

		var err error

		// Perform validation based on value type
		switch cv := vField.Interface().(type) {
		case int:
			min, max := readMinMax(validationTag)
			err = ValidateIntMinMaxField(int64(cv), int64(min), int64(max))
		case int64:
			var keep bool
			keep, err = ValidateIntField(cv, validationTag, guild)
			if err == nil && !keep {
				vField.SetInt(0)
			}
		case float64:
			min, max := readMinMax(validationTag)
			err = ValidateFloatField(cv, min, max)
		case float32:
			min, max := readMinMax(validationTag)
			err = ValidateFloatField(float64(cv), min, max)
		case string:
			var newS string
			newS, err = ValidateStringField(cv, validationTag, guild)
			if err == nil {
				vField.SetString(newS)
			}
		case []string:
			newSlice := make([]string, 0, len(cv))
			for _, s := range cv {
				newS, e := ValidateStringField(s, validationTag, guild)
				if e != nil {
					err = e
					break
				}

				if newS != "" && !common.ContainsStringSlice(newSlice, newS) {
					newSlice = append(newSlice, newS)
				}
			}
			vField.Set(reflect.ValueOf(newSlice))
		case []int64:
			newSlice := make([]int64, 0, len(cv))
			for _, integer := range cv {
				keep, e := ValidateIntField(integer, validationTag, guild)
				if e != nil {
					err = e
					break
				}
				if keep && !common.ContainsInt64Slice(newSlice, integer) {
					newSlice = append(newSlice, integer)
				}
			}

			vField.Set(reflect.ValueOf(newSlice))
		default:
			// Recurse if it's another struct
			switch tField.Type.Kind() {
			case reflect.Struct, reflect.Ptr:
				addr := reflect.Indirect(vField).Addr()
				innerOk := ValidateForm(guild, tmpl, addr.Interface())
				if !innerOk {
					ok = false
				}
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

	return ok
}

func readMinMax(valid *ValidationTag) (float64, float64) {

	min, _ := valid.Float(0)
	max, _ := valid.Float(1)

	return min, max
}

func ValidateIntField(i int64, tags *ValidationTag, guild *discordgo.Guild) (keep bool, err error) {
	kind, _ := tags.Str(0)

	if kind != "role" && kind != "channel" {
		// Treat as min max
		min, max := readMinMax(tags)
		return true, ValidateIntMinMaxField(i, int64(min), int64(max))
	}

	if kind == "" {
		return true, nil
	}

	// Treat any non empty and non-"false" true
	allowEmpty := false
	if allow, ok := tags.Str(1); ok {
		if strings.ToLower(allow) != "false" && allow != "-" && allow != "" {
			allowEmpty = true
		}
	}

	// Check what kind of string field it is, and perform the needed vliadation depending on type
	switch kind {
	case "role":
		err = ValidateRoleField(strconv.FormatInt(i, 10), guild.Roles, allowEmpty)
	case "channel":
		err = ValidateChannelField(strconv.FormatInt(i, 10), guild.Channels, allowEmpty)
	default:
		logrus.WithField("kind", kind).Error("UNKNOWN INT TYPE IN VALIDATION! (typo maybe?)")
	}

	if err != nil && allowEmpty {
		return false, nil
	}

	return true, err

}

func ValidateIntMinMaxField(i int64, min, max int64) error {

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
		return fmt.Errorf("Too long (max %d)", max)
	}

	_, err := regexp.Compile(s)
	return err
}

func ValidateStringField(s string, tags *ValidationTag, guild *discordgo.Guild) (str string, err error) {
	maxLen := 2000

	str = s

	kind, _ := tags.Str(0)

	// Retrieve max len from tag is needed
	if kind == "template" || kind == "regex" || kind == "" {

		m, ok := tags.Int(1)
		if ok {
			maxLen = m
		}
	}

	// Treat any non empty and non-"false" true
	allowEmpty := false
	if allow, ok := tags.Str(1); ok {
		if strings.ToLower(allow) != "false" && allow != "-" && allow != "" {
			allowEmpty = true
		}
	}

	// Check what kind of string field it is, and perform the needed vliadation depending on type
	switch kind {
	case "template":
		err = ValidateTemplateField(s, maxLen)
	case "regex":
		err = ValidateRegexField(s, maxLen)
	case "role":
		err = ValidateRoleField(s, guild.Roles, allowEmpty)
		if err != nil && allowEmpty {
			str = ""
			err = nil
		}
	case "channel":
		err = ValidateChannelField(s, guild.Channels, allowEmpty)
		if err != nil && allowEmpty {
			str = ""
			err = nil
		}
	case "":
		min := -1

		startIndex := 2
		// If only 1 argument provided, its max, if 2 then it's min,max
		if newMax, ok := tags.Int(2); ok {
			min = maxLen
			maxLen = newMax
			startIndex = 3
		}

		for i := startIndex; i < len(tags.values); i++ {
			t, ok := tags.Str(i)
			if !ok {
				return str, errors.New("Failed reading tag: " + str)
			}
			switch t {
			case "trimspace":
				str = strings.TrimSpace(str)
			}
		}

		err = ValidateNormalStringField(s, min, maxLen)
	default:
		logrus.WithField("kind", kind).Error("UNKNOWN STRING TYPE IN VALIDATION! (typo maybe?)")
	}

	return str, err
}

func ValidateNormalStringField(s string, min, max int) error {
	rCount := utf8.RuneCountInString(s)
	if rCount > max {
		return fmt.Errorf("Too long (max %d)", max)
	}

	if rCount < min {
		return fmt.Errorf("Too short (min %d)", min)
	}

	return nil
}

func ValidateTemplateField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return fmt.Errorf("Too long (max %d)", max)
	}

	// _, err := common.ParseExecuteTemplate(s, nil)
	return nil
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
