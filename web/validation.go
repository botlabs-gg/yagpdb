package web

// Form validation tools
// Pass it a struct and it will validate each field
// depending on struct tags
//
// struct: `valid:"traverse"`
//	  - Validates the struct or slice
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
//
// if the struct also implements CustomValidator then that will also be ran
import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/botlabs-gg/yagpdb/common"
	"github.com/botlabs-gg/yagpdb/common/templates"
	"github.com/jonas747/discordgo/v2"
	"github.com/jonas747/dstate/v4"
	"github.com/lib/pq"
)

type CustomValidator interface {
	Validate(ctx *ValidationContext)
}

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
	ErrChannelNotFound = errors.New("channel not found")
	ErrRoleNotFound    = errors.New("role not found")
)

type ValidationError struct {
	Path   string `json:"path"`
	PathGo string `json:"path_go"`
	Err    string `json:"err"`
}

type StructPath struct {
	field    reflect.StructField
	index    int
	hasIndex bool
}

type ValidationContext struct {
	field_errors   []ValidationError
	general_errors []string
	guild          *dstate.GuildSet
	form           interface{}
	currenPath     []*StructPath
}

type ValidationResult struct {
	FieldErrors   []ValidationError `json:"field_errors"`
	GeneralErrors []string          `json:"general_errors"`
}

func (vr *ValidationResult) IsOK() bool {
	return len(vr.FieldErrors) == 0 && len(vr.GeneralErrors) == 0
}

func (vr *ValidationResult) AddToTemplate(tmpl TemplateData) {
	for _, vErr := range vr.GeneralErrors {
		tmpl.AddAlerts(ErrorAlert(vErr))
	}

	for _, vErr := range vr.FieldErrors {

		prettyField := ""
		for _, r := range vErr.PathGo {
			if unicode.IsUpper(r) {
				if prettyField != "" {
					prettyField += " "
				}
			}

			if r == '.' {
				prettyField += " ->"
			} else {
				prettyField += string(r)
			}

		}
		prettyField = strings.TrimSpace(prettyField)
		tmpl.AddAlerts(ErrorAlert(prettyField, ": ", vErr.Err))
	}
}

// Probably needs some cleaning up
func ValidateForm(guild *dstate.GuildSet, form interface{}) ValidationResult {
	ctx := &ValidationContext{
		guild: guild,
		form:  form,
	}
	ctx.validateForm(form)

	return ValidationResult{
		FieldErrors:   ctx.field_errors,
		GeneralErrors: ctx.general_errors,
	}
}

func (vctx *ValidationContext) validateForm(current interface{}) {
	v := reflect.Indirect(reflect.ValueOf(current))
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
			keep, err = ValidateIntField(cv, validationTag, vctx.guild, false)
			if err == nil && !keep {
				vField.SetInt(0)
			}
		case sql.NullInt64:
			var keep bool
			var newNullInt sql.NullInt64
			keep, err = ValidateIntField(cv.Int64, validationTag, vctx.guild, false)
			if err == nil && !keep {
				vField.Set(reflect.ValueOf(newNullInt))
			}
		case float64:
			min, max := readMinMax(validationTag)
			err = ValidateFloatField(cv, min, max)
		case float32:
			min, max := readMinMax(validationTag)
			err = ValidateFloatField(float64(cv), min, max)
		case string:
			var newS string
			newS, err = ValidateStringField(cv, validationTag, vctx.guild)
			if err == nil {
				vField.SetString(newS)
			}
		case []string:
			newSlice := make([]string, 0, len(cv))
			for _, s := range cv {
				newS, e := ValidateStringField(s, validationTag, vctx.guild)
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
			newSlice, e := ValidateIntSliceField(cv, validationTag, vctx.guild)
			if e != nil {
				err = e
				break
			}

			vField.Set(reflect.ValueOf(newSlice))
		case pq.Int64Array:
			newSlice, e := ValidateIntSliceField(cv, validationTag, vctx.guild)
			if e != nil {
				err = e
				break
			}

			vField.Set(reflect.ValueOf(pq.Int64Array(newSlice)))
		default:
			// Recurse if it's another struct
			switch tField.Type.Kind() {
			case reflect.Struct, reflect.Ptr:
				addr := reflect.Indirect(vField).Addr()

				vctx.PushPath(tField)
				vctx.validateForm(addr.Interface())
				vctx.PopPath()

			case reflect.Slice:
				// validate all slice elements
				sl := reflect.Indirect(vField)
				for i := 0; i < sl.Len(); i++ {
					vctx.PushPathWithIndex(tField, i)
					vctx.validateForm(sl.Index(i).Addr().Interface())
					vctx.PopPath()
				}
			}
		}

		if err != nil {

			// Create a pretty name for the field by turing: "AnnounceMessage" into "Announce Message"
			// prettyField := ""
			// for _, r := range tField.Name {
			// 	if unicode.IsUpper(r) {
			// 		if prettyField != "" {
			// 			prettyField += " "
			// 		}
			// 	}

			// 	prettyField += string(r)
			// }
			// prettyField = strings.TrimSpace(prettyField)

			// tmpl.AddAlerts(ErrorAlert(prettyField, ": ", err.Error()))
			// ok = false
			vctx.PushFieldError(tField, err)
		}
	}

	if validator, okc := current.(CustomValidator); okc {
		validator.Validate(vctx)
	}
}

func (vctx *ValidationContext) StringPath(seperator string) string {
	path := ""
	for i, v := range vctx.currenPath {
		if i != 0 {
			path += seperator
		}

		path += v.field.Name
		if v.hasIndex {
			path += "[" + strconv.Itoa(v.index) + "]"
		}
	}

	return path
}

func (vctx *ValidationContext) StringPathJson() string {
	path := ""
	for i, v := range vctx.currenPath {
		if i != 0 {
			path += "."
		}

		path += JsonTagName(v.field.Name, v.field.Tag.Get("json"))
		if v.hasIndex {
			path += "[" + strconv.Itoa(v.index) + "]"
		}
	}

	return path
}

func JsonTagName(fieldName string, jsonTag string) string {
	if jsonTag == "" {
		return fieldName
	}

	split := strings.Split(jsonTag, ",")
	return split[0]
}

func (vctx *ValidationContext) PushError(err error) {
	vctx.general_errors = append(vctx.general_errors, err.Error())
}

func (vctx *ValidationContext) PushFieldError(field reflect.StructField, err error) {
	pathPrefix := vctx.StringPath(".")
	if pathPrefix != "" {
		pathPrefix += "."
	}

	pathPrefixJson := vctx.StringPathJson()
	if pathPrefixJson != "" {
		pathPrefixJson += "."
	}

	vctx.field_errors = append(vctx.field_errors, ValidationError{
		PathGo: pathPrefix + field.Name,
		Path:   pathPrefixJson + JsonTagName(field.Name, string(field.Tag.Get("json"))),
		Err:    err.Error(),
	})
}

func (vctx *ValidationContext) PushFieldErrorWithIndex(field reflect.StructField, index int, err error) {
	pathPrefix := vctx.StringPath(".")
	if pathPrefix != "" {
		pathPrefix += "."
	}

	pathPrefixJson := vctx.StringPathJson()
	if pathPrefixJson != "" {
		pathPrefixJson += "."
	}

	vctx.field_errors = append(vctx.field_errors, ValidationError{
		PathGo: pathPrefix + field.Name + "[" + strconv.Itoa(index) + "]",
		Path:   pathPrefix + JsonTagName(field.Name, field.Tag.Get("json")+"["+strconv.Itoa(index)+"]"),
		Err:    err.Error(),
	})
}

func (vctx *ValidationContext) PushPath(field reflect.StructField) {
	vctx.currenPath = append(vctx.currenPath, &StructPath{
		field: field,
	})
}

func (vctx *ValidationContext) PushPathWithIndex(field reflect.StructField, index int) {
	vctx.currenPath = append(vctx.currenPath, &StructPath{
		field:    field,
		index:    index,
		hasIndex: true,
	})
}

func (vctx *ValidationContext) PopPath() {
	vctx.currenPath = vctx.currenPath[:len(vctx.currenPath)-1]
}

func readMinMax(valid *ValidationTag) (float64, float64) {

	min, _ := valid.Float(0)
	max, _ := valid.Float(1)

	return min, max
}

func ValidateIntSliceField(is []int64, tags *ValidationTag, guild *dstate.GuildSet) (filtered []int64, err error) {
	filtered = make([]int64, 0, len(is))
	for _, integer := range is {
		keep, err := ValidateIntField(integer, tags, guild, true)
		if err != nil {
			return filtered, err
		}

		if keep && !common.ContainsInt64Slice(filtered, integer) {
			filtered = append(filtered, integer)
		}
	}

	return filtered, nil
}

func ValidateIntField(i int64, tags *ValidationTag, guild *dstate.GuildSet, forceAllowEmpty bool) (keep bool, err error) {
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
	allowEmpty := forceAllowEmpty
	if !allowEmpty {
		if allow, ok := tags.Str(1); ok {
			if strings.ToLower(allow) != "false" && allow != "-" && allow != "" {
				allowEmpty = true
			}
		}
	}

	// Check what kind of string field it is, and perform the needed vliadation depending on type
	switch kind {
	case "role":
		err = ValidateRoleField(i, guild.Roles, allowEmpty)
	case "channel":
		err = ValidateChannelField(i, guild.Channels, allowEmpty)
	default:
		logger.WithField("kind", kind).Error("UNKNOWN INT TYPE IN VALIDATION! (typo maybe?)")
	}

	if (err != nil || i == 0) && allowEmpty {
		return false, nil
	}

	return true, err

}

func ValidateIntMinMaxField(i int64, min, max int64) error {

	if min != max && (i < min || i > max) {
		return fmt.Errorf("out of range (%d - %d)", min, max)
	}

	return nil
}

func ValidateFloatField(f float64, min, max float64) error {

	if min != max && (f < min || f > max) {
		return fmt.Errorf("out of range (%f - %f)", min, max)
	}

	return nil
}

func ValidateRegexField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return fmt.Errorf("too long (max %d)", max)
	}

	_, err := regexp.Compile(s)
	return err
}

func ValidateStringField(s string, tags *ValidationTag, guild *dstate.GuildSet) (str string, err error) {
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
		parsedID, _ := strconv.ParseInt(s, 10, 64)
		err = ValidateRoleField(parsedID, guild.Roles, allowEmpty)
		if err != nil && allowEmpty {
			str = ""
			err = nil
		}
	case "channel":
		parsedID, _ := strconv.ParseInt(s, 10, 64)
		err = ValidateChannelField(parsedID, guild.Channels, allowEmpty)
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

		err = ValidateNormalStringField(str, min, maxLen)
	default:
		logger.WithField("kind", kind).Error("UNKNOWN STRING TYPE IN VALIDATION! (typo maybe?)")
	}

	return str, err
}

func ValidateNormalStringField(s string, min, max int) error {
	rCount := utf8.RuneCountInString(s)
	if rCount > max {
		return fmt.Errorf("too long (max %d)", max)
	}

	if rCount < min {
		return fmt.Errorf("too short (min %d)", min)
	}

	return nil
}

func ValidateTemplateField(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return fmt.Errorf("too long (max %d)", max)
	}

	_, err := templates.NewContext(nil, nil, nil).Parse(s)
	return err
}

func ValidateChannelField(s int64, channels []dstate.ChannelState, allowEmpty bool) error {
	if s == 0 {
		if allowEmpty {
			return nil
		} else {
			return errors.New("no channel specified")
		}
	}

	for _, v := range channels {
		if s == v.ID {
			return nil
		}
	}

	return ErrChannelNotFound
}

func ValidateRoleField(s int64, roles []discordgo.Role, allowEmpty bool) error {
	if s == 0 {
		if allowEmpty {
			return nil
		} else {
			return errors.New("no role specified (or role is above bot)")
		}
	}

	for _, v := range roles {
		if s == v.ID {
			return nil
		}
	}

	return ErrRoleNotFound
}
