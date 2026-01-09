package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

// dictionary creates a map[string]interface{} from the given parameters by
// walking the parameters and treating them as key-value pairs.  The number
// of parameters must be even.
func Dictionary(values ...interface{}) (Dict, error) {

	if len(values) == 1 {
		val, isNil := indirect(reflect.ValueOf(values[0]))
		if isNil || values[0] == nil {
			return nil, errors.New("dict: nil value passed")
		}

		if Dict, ok := val.Interface().(Dict); ok {
			return Dict, nil
		}

		switch val.Kind() {
		case reflect.Map:
			iter := val.MapRange()
			mapCopy := make(map[interface{}]interface{})
			for iter.Next() {
				mapCopy[iter.Key().Interface()] = iter.Value().Interface()
			}
			return Dict(mapCopy), nil
		default:
			return nil, errors.New("cannot convert data of type: " + reflect.TypeOf(values[0]).String())
		}

	}

	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}

	dict := make(map[interface{}]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key := values[i]
		dict[key] = values[i+1]
	}

	return Dict(dict), nil
}

func StringKeyDictionary(values ...interface{}) (SDict, error) {

	if len(values) == 1 {
		val, isNil := indirect(reflect.ValueOf(values[0]))
		if isNil || values[0] == nil {
			return nil, errors.New("Sdict: nil value passed")
		}

		if sdict, ok := val.Interface().(SDict); ok {
			return sdict, nil
		}

		switch val.Kind() {
		case reflect.Map:
			iter := val.MapRange()
			mapCopy := make(map[string]interface{})
			for iter.Next() {

				key, isNil := indirect(iter.Key())
				if isNil {
					return nil, errors.New("map with nil key encountered")
				}
				if key.Kind() == reflect.String {
					mapCopy[key.String()] = iter.Value().Interface()
				} else {
					return nil, errors.New("map has non string key of type: " + key.Type().String())
				}
			}
			return SDict(mapCopy), nil
		default:
			return nil, errors.New("cannot convert data of type: " + reflect.TypeOf(values[0]).String())
		}

	}

	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key := values[i]
		s, ok := key.(string)
		if !ok {
			return nil, errors.New("Only string keys supported in sdict")
		}

		dict[s] = values[i+1]
	}

	return SDict(dict), nil
}

// CreateComponentBuilder parses given input of key-value pairs but returns the
// keys and the values as separate slices. this is used when you need to have
// duplicate keys and therefore cannot use a map.
func CreateComponentBuilder(values ...interface{}) (compBuilder *ComponentBuilder, err error) {
	compBuilder = &ComponentBuilder{}

	if len(values) == 1 {
		val, isNil := indirect(reflect.ValueOf(values[0]))
		if isNil || values[0] == nil {
			err = errors.New("Sdict: nil value passed")
			return
		}

		switch typed := val.Interface().(type) {
		case *ComponentBuilder:
			return typed, nil
		case ComponentBuilder:
			return &typed, nil
		}

		switch val.Kind() {
		case reflect.Map:
			iter := val.MapRange()
			for iter.Next() {

				key, isNil := indirect(iter.Key())
				if isNil {
					err = errors.New("map with nil key encountered")
					return
				}
				if key.Kind() == reflect.String {
					compBuilder.Components = append(compBuilder.Components, key.String())
					compBuilder.Values = append(compBuilder.Values, iter.Value().Interface())
				} else {
					err = errors.New("map has non string key of type: " + key.Type().String())
					return
				}
			}
			return
		default:
			err = errors.New("cannot convert data of type: " + reflect.TypeOf(values[0]).String())
			return
		}

	}

	if len(values)%2 != 0 {
		err = errors.New("invalid dict call")
		return
	}
	for i := 0; i < len(values); i += 2 {
		key := values[i]
		s, ok := key.(string)
		if !ok {
			err = errors.New("Only string keys supported in sdict")
			return
		}

		compBuilder.Components = append(compBuilder.Components, s)
		compBuilder.Values = append(compBuilder.Values, values[i+1])
	}
	return
}

func KindOf(input interface{}, flag ...bool) (string, error) { //flag used only for indirect vs direct for now.

	switch len(flag) {

	case 0:
		return reflect.ValueOf(input).Kind().String(), nil
	case 1:
		if flag[0] {
			val, isNil := indirect(reflect.ValueOf(input))
			if isNil || input == nil {
				return "invalid", nil
			}
			return val.Kind().String(), nil
		}
		return reflect.ValueOf(input).Kind().String(), nil
	default:
		return "", errors.New("Too many flags")
	}
}

func StructToSdict(value interface{}) (SDict, error) {
	val, isNil := indirect(reflect.ValueOf(value))
	typeOfS := val.Type()
	if isNil || value == nil {
		return nil, errors.New("Expected - struct, got - Nil ")
	}

	if val.Kind() != reflect.Struct {
		return nil, errors.New(fmt.Sprintf("Expected - struct, got - %s", val.Type().String()))
	}

	fields := make(map[string]interface{})
	for i := 0; i < val.NumField(); i++ {
		curr := val.Field(i)
		if curr.CanInterface() {
			fields[typeOfS.Field(i).Name] = curr.Interface()
		}
	}
	return SDict(fields), nil

}

func CreateSlice(values ...interface{}) (Slice, error) {
	slice := make([]interface{}, len(values))
	copy(slice, values)
	return Slice(slice), nil
}

func CreateEmbed(values ...interface{}) (*discordgo.MessageEmbed, error) {
	if len(values) < 1 {
		return &discordgo.MessageEmbed{}, nil
	}

	var m map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		m = t
	case *SDict:
		m = *t
	case map[string]interface{}:
		m = t
	case *discordgo.MessageEmbed:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		m = dict
	}

	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var embed *discordgo.MessageEmbed
	err = json.Unmarshal(encoded, &embed)
	if err != nil {
		return nil, err
	}

	return embed, nil
}

func CreateMessageSend(values ...interface{}) (*discordgo.MessageSend, error) {
	if len(values) < 1 {
		return &discordgo.MessageSend{}, nil
	}

	if m, ok := values[0].(*discordgo.MessageSend); len(values) == 1 && ok {
		return m, nil
	}

	compBuilder, err := CreateComponentBuilder(values...)
	if err != nil {
		return nil, err
	}

	msg := &discordgo.MessageSend{
		AllowedMentions: discordgo.AllowedMentions{
			Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
		},
	}

	// Default filename
	filename := "attachment_" + time.Now().Format("2006-01-02_15-04-05")
	for i, key := range compBuilder.Components {
		val := compBuilder.Values[i]

		switch strings.ToLower(key) {
		case "content":
			msg.Content = ToString(val)
		case "embed":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				const maxEmbeds = 10 // Discord limitation
				for i := 0; i < v.Len() && i < maxEmbeds; i++ {
					embed, err := CreateEmbed(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					msg.Embeds = append(msg.Embeds, embed)
				}
			} else {
				embed, err := CreateEmbed(val)
				if err != nil {
					return nil, err
				}
				msg.Embeds = append(msg.Embeds, embed)
			}
		case "file":
			stringFile := ToString(val)
			if len(stringFile) > 100000 {
				return nil, errors.New("file length for send message builder exceeded size limit")
			}
			var buf bytes.Buffer
			buf.WriteString(stringFile)

			msg.File = &discordgo.File{
				ContentType: "text/plain",
				Reader:      &buf,
			}
		case "allowed_mentions":
			if val == nil {
				msg.AllowedMentions = discordgo.AllowedMentions{}
				continue
			}
			parsed, err := parseAllowedMentions(val)
			if err != nil {
				return nil, err
			}
			msg.AllowedMentions = *parsed
		case "filename":
			// Cut the filename to a reasonable length if it's too long
			filename = common.CutStringShort(ToString(val), 64)
		case "reply":
			msgID := ToInt64(val)
			if msgID <= 0 {
				return nil, errors.New(fmt.Sprintf("invalid message id '%s' provided to reply.", ToString(val)))
			}
			msg.Reference = &discordgo.MessageReference{
				MessageID: msgID,
			}
		case "silent":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsSuppressNotifications
		case "components":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				msg.Components, err = distributeComponentsIntoActionsRows(v)
				if err != nil {
					return nil, err
				}
			} else {
				var component discordgo.InteractiveComponent
				switch comp := val.(type) {
				case *discordgo.SelectMenu:
					component = comp
				case *discordgo.Button:
					component = comp
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{component}})
			}
		case "ephemeral":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsEphemeral
		case "buttons":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				buttons := []*discordgo.Button{}
				const maxButtons = 40 // Discord limitation
				for i := 0; i < v.Len() && i < maxButtons; i++ {
					button, err := CreateButton(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					buttons = append(buttons, button)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(buttons))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				button, err := CreateButton(val)
				if err != nil {
					return nil, err
				}
				if button.Style == discordgo.LinkButton {
					button.CustomID = ""
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{button}})
			}
		case "menus":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				menus := []*discordgo.SelectMenu{}
				const maxMenus = 5 // Discord limitation
				for i := 0; i < v.Len() && i < maxMenus; i++ {
					menu, err := CreateSelectMenu(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					menus = append(menus, menu)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(menus))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				menu, err := CreateSelectMenu(val)
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{menu}})
			}
		case "forward":
			if val == nil {
				continue
			}
			var m map[string]interface{}
			switch t := val.(type) {
			case SDict:
				m = t
			case *SDict:
				m = *t
			case map[string]interface{}:
				m = t
			default:
				return nil, errors.New("invalid value passed to forward; must be an sdict with channel and message")
			}

			msg.Reference = &discordgo.MessageReference{
				Type: 1,
			}
			for k, v := range m {
				switch strings.ToLower(k) {
				case "channel":
					msg.Reference.ChannelID = ToInt64(v)
					if msg.Reference.ChannelID <= 0 {
						return nil, errors.New(fmt.Sprintf("invalid channel id '%s' provided to forward.", ToString(val)))
					}
				case "message":
					msg.Reference.MessageID = ToInt64(v)
					if msg.Reference.MessageID <= 0 {
						return nil, errors.New(fmt.Sprintf("invalid message id '%s' provided to forward.", ToString(val)))
					}
				}
			}
		case "sticker":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				const maxStickers = 3 // Discord limitation
				for i := 0; i < v.Len() && i < maxStickers; i++ {
					msg.StickerIDs = append(msg.StickerIDs, ToInt64(v.Index(i).Interface()))
				}
			} else {
				msg.StickerIDs = append(msg.StickerIDs, ToInt64(val))
			}
		case "suppress_embeds":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsSuppressEmbeds
		case "is_components_v2":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsIsComponentsV2
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to send message builder.`)
		}

	}
	if msg.File != nil {
		// We hardcode the extension to .txt to prevent possible abuse via .bat or other possible harmful/easily corruptable file formats
		msg.File.Name = filename + ".txt"
	}

	if len(msg.Components) > 0 {
		err := validateTopLevelComponentsCustomIDs(msg.Components, nil)
		if err != nil {
			return nil, err
		}
	}

	return msg, nil
}

func CreateMessageEdit(values ...interface{}) (*discordgo.MessageEdit, error) {
	if len(values) < 1 {
		return &discordgo.MessageEdit{}, nil
	}

	if m, ok := values[0].(*discordgo.MessageEdit); len(values) == 1 && ok {
		return m, nil
	}

	compBuilder, err := CreateComponentBuilder(values...)
	if err != nil {
		return nil, err
	}

	msg := &discordgo.MessageEdit{}
	for i, key := range compBuilder.Components {
		val := compBuilder.Values[i]
		switch strings.ToLower(key) {
		case "content":
			temp := fmt.Sprint(val)
			msg.Content = &temp
		case "embed":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				const maxEmbeds = 10 // Discord limitation
				for i := 0; i < v.Len() && i < maxEmbeds; i++ {
					embed, err := CreateEmbed(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					msg.Embeds = append(msg.Embeds, embed)
				}
			} else {
				embed, err := CreateEmbed(val)
				if err != nil {
					return nil, err
				}
				msg.Embeds = append(msg.Embeds, embed)
			}
		case "silent":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsSuppressNotifications
		case "allowed_mentions":
			if val == nil {
				msg.AllowedMentions = discordgo.AllowedMentions{}
				continue
			}
			parsed, err := parseAllowedMentions(val)
			if err != nil {
				return nil, err
			}
			msg.AllowedMentions = *parsed
		case "components":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				msg.Components, err = distributeComponentsIntoActionsRows(v)
				if err != nil {
					return nil, err
				}
			} else {
				var component discordgo.InteractiveComponent
				switch comp := val.(type) {
				case *discordgo.SelectMenu:
					component = comp
				case *discordgo.Button:
					component = comp
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{component}})
			}
		case "buttons":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				buttons := []*discordgo.Button{}
				const maxButtons = 25 // Discord limitation
				for i := 0; i < v.Len() && i < maxButtons; i++ {
					button, err := CreateButton(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					buttons = append(buttons, button)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(buttons))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				button, err := CreateButton(val)
				if err != nil {
					return nil, err
				}
				if button.Style == discordgo.LinkButton {
					button.CustomID = ""
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{button}})
			}
		case "menus":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				menus := []*discordgo.SelectMenu{}
				const maxMenus = 5 // Discord limitation
				for i := 0; i < v.Len() && i < maxMenus; i++ {
					menu, err := CreateSelectMenu(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					menus = append(menus, menu)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(menus))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				menu, err := CreateSelectMenu(val)
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{menu}})
			}
		case "suppress_embeds":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsSuppressEmbeds
		case "is_components_v2":
			if val == nil || val == false {
				continue
			}
			msg.Flags |= discordgo.MessageFlagsIsComponentsV2
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to message edit builder`)
		}

	}

	if len(msg.Components) > 0 {
		err := validateTopLevelComponentsCustomIDs(msg.Components, nil)
		if err != nil {
			return nil, err
		}
	}

	return msg, nil

}

func parseAllowedMentions(Data interface{}) (*discordgo.AllowedMentions, error) {

	if m, ok := Data.(discordgo.AllowedMentions); ok {
		return &m, nil
	}

	converted, err := StringKeyDictionary(Data)
	if err != nil {
		return nil, err
	}

	var parsingUsers bool
	var parsingRoles bool

	allowedMentions := &discordgo.AllowedMentions{}
	for k, v := range converted {

		switch strings.ToLower(k) {
		case "parse":
			var parseMentions []discordgo.AllowedMentionType
			var parseSlice Slice
			conv, err := parseSlice.AppendSlice(v)
			if err != nil {
				return nil, errors.New(`Allowed Mentions Parsing: invalid datatype passed to "Parse", accepts a slice only`)
			}
			for _, elem := range conv.(Slice) {
				elem_conv, _ := elem.(string)
				if elem_conv != "users" && elem_conv != "roles" && elem_conv != "everyone" {
					return nil, errors.New(`Allowed Mentions Parsing: invalid slice element in "Parse", accepts "roles", "users", and "everyone"`)
				}
				parseMentions = append(parseMentions, discordgo.AllowedMentionType(elem_conv))
				if elem_conv == "users" {
					parsingUsers = true
				} else if elem_conv == "roles" {
					parsingRoles = true
				}
			}
			allowedMentions.Parse = parseMentions
		case "users", "roles":
			var newslice discordgo.IDSlice
			var parseSlice Slice
			conv, err := parseSlice.AppendSlice(v)
			if err != nil {
				return nil, fmt.Errorf(`allowed Mentions Parsing: invalid datatype passed to "%s", accepts a slice of snowflakes only`, k)
			}
			for _, elem := range conv.(Slice) {
				if (ToInt64(elem)) == 0 {
					return nil, fmt.Errorf(`allowed Mentions Parsing: "%s" IDSlice: invalid ID passed -`+fmt.Sprint(elem), k)
				}
				newslice = append(newslice, ToInt64(elem))
			}
			if len(newslice) > 100 {
				newslice = newslice[:100]
			}
			if strings.ToLower(k) == "users" {
				allowedMentions.Users = newslice
			} else {
				allowedMentions.Roles = newslice
			}
		case "replied_user":
			isRepliedUserMention, ok := v.(bool)
			if !ok {
				return nil, errors.New(`Allowed Mentions Parsing: invalid datatype passed to "replied_user", accepts a bool only`)
			}
			allowedMentions.RepliedUser = isRepliedUserMention
		default:
			return nil, errors.New(`Allowed Mentions Parsing: invalid key "` + k + `" for Allowed Mentions`)
		}
	}

	if parsingUsers && allowedMentions.Users != nil {
		return nil, errors.New(`Allowed Mentions Parsing: conflicting values passed, you cannot parse all users if only allowing a set of users`)
	} else if parsingRoles && allowedMentions.Roles != nil {
		return nil, errors.New(`Allowed Mentions Parsing: conflicting values passed, you cannot parse all roles if only allowing a set of roles`)
	}

	return allowedMentions, nil
}

// indirect is taken from 'text/template/exec.go'
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
		if v.Kind() == reflect.Interface && v.NumMethod() > 0 {
			break
		}
	}
	return v, false
}

// in returns whether v is in the set l.  l may be an array or slice.
func in(l interface{}, v interface{}) bool {
	lv, _ := indirect(reflect.ValueOf(l))
	vv := reflect.ValueOf(v)

	if reflect.ValueOf(vv).IsZero() {
		return false
	}

	switch lv.Kind() {
	case reflect.String:
		if vv.Type() == lv.Type() && strings.Contains(lv.String(), vv.String()) {
			return true
		}
	case reflect.Array, reflect.Slice:
		for i := range lv.Len() {
			lvv := lv.Index(i)
			lvv, isNil := indirect(lvv)
			if isNil {
				continue
			}
			switch {
			case lvv.Kind() == reflect.String:
				if vv.Type() == lvv.Type() && vv.String() == lvv.String() {
					return true
				}
			case lvv.CanInt() && vv.CanInt():
				if vv.Int() == lvv.Int() {
					return true
				}
			case lvv.CanUint() && vv.CanUint():
				if vv.Uint() == lvv.Uint() {
					return true
				}
			case lvv.CanFloat() && vv.CanFloat():
				if vv.Float() == lvv.Float() {
					return true
				}
			}
		}
	}

	return false
}

// in returns whether v is in the set l. l may only be a slice of strings, or a string, v may only be a string
// it differs from "in" because its case insensitive
func inFold(l interface{}, v string) bool {
	lv, _ := indirect(reflect.ValueOf(l))
	vv := reflect.ValueOf(v)

	switch lv.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < lv.Len(); i++ {
			lvv := lv.Index(i)
			lvv, isNil := indirect(lvv)
			if isNil {
				continue
			}
			switch lvv.Kind() {
			case reflect.String:
				if vv.Type() == lvv.Type() && strings.EqualFold(vv.String(), lvv.String()) {
					return true
				}
			}
		}
	case reflect.String:
		if vv.Type() == lv.Type() && strings.Contains(strings.ToLower(lv.String()), strings.ToLower(vv.String())) {
			return true
		}
	}

	return false
}

func tmplAbs(arg interface{}) interface{} {
	absF := math.Abs(ToFloat64(arg))

	switch arg.(type) {
	case float32, float64:
		return absF
	default:
		return int(absF)
	}
}

func add(args ...interface{}) interface{} {
	if len(args) < 1 {
		return 0
	}

	switch args[0].(type) {
	case float32, float64:
		sumF := float64(0)
		for _, v := range args {
			sumF += ToFloat64(v)
		}
		return sumF
	default:
		sumI := 0
		for _, v := range args {
			sumI += tmplToInt(v)
		}
		return sumI
	}
}

func tmplSub(args ...interface{}) interface{} {
	if len(args) < 1 {
		return 0
	}

	switch args[0].(type) {
	case float32, float64:
		subF := ToFloat64(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}
			subF -= ToFloat64(v)
		}
		return subF
	default:
		subI := tmplToInt(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}
			subI -= tmplToInt(v)
		}
		return subI
	}
}

var mathConstantsMap = map[string]float64{
	//base
	"e":   math.E,
	"pi":  math.Pi,
	"phi": math.Phi,

	// square roots
	"sqrt2":   math.Sqrt2,
	"sqrte":   math.SqrtE,
	"sqrtpi":  math.SqrtPi,
	"sqrtphi": math.SqrtPhi,

	// logarithms
	"ln2":    math.Ln2,
	"log2e":  math.Log2E,
	"ln10":   math.Ln10,
	"log10e": math.Log10E,

	// floating-point limit values
	"maxfloat32":             math.MaxFloat32,
	"smallestnonzerofloat32": math.SmallestNonzeroFloat32,
	"maxfloat64":             math.MaxFloat64,
	"smallestnonzerofloat64": math.SmallestNonzeroFloat64,

	// integer limit values
	"maxint":    math.MaxInt,
	"minint":    math.MinInt,
	"maxint8":   math.MaxInt8,
	"minint8":   math.MinInt8,
	"maxint16":  math.MaxInt16,
	"minint16":  math.MinInt16,
	"maxint32":  math.MaxInt32,
	"minint32":  math.MinInt32,
	"maxint64":  math.MaxInt64,
	"minint64":  math.MinInt64,
	"maxuint":   math.MaxUint,
	"maxuint8":  math.MaxUint8,
	"maxuint16": math.MaxUint16,
	"maxuint32": math.MaxUint32,
	"maxuint64": math.MaxUint64,
}

func tmplMathConstant(arg string) float64 {
	constant := mathConstantsMap[strings.ToLower(arg)]
	if constant == 0 {
		return math.NaN()
	}

	return constant
}

func tmplMult(args ...interface{}) interface{} {
	if len(args) < 1 {
		return 0
	}

	switch args[0].(type) {
	case float32, float64:
		sumF := ToFloat64(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}

			sumF *= ToFloat64(v)
		}
		return sumF
	default:
		sumI := tmplToInt(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}

			sumI *= tmplToInt(v)
		}
		return sumI
	}
}

func tmplDiv(args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return 0, nil
	}

	switch args[0].(type) {
	case float32, float64:
		sumF := ToFloat64(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}

			sumF /= ToFloat64(v)
		}
		return sumF, nil
	default:
		sumI := tmplToInt(args[0])
		for i, v := range args {
			if i == 0 {
				continue
			}
			if tmplToInt(v) == 0 {
				return 0, errors.New("integer divide by zero")
			}

			sumI /= tmplToInt(v)
		}
		return sumI, nil
	}
}

func tmplMod(args ...interface{}) float64 {
	if len(args) != 2 {
		return math.NaN()
	}

	return math.Mod(ToFloat64(args[0]), ToFloat64(args[1]))
}

func tmplFDiv(args ...interface{}) interface{} {
	if len(args) < 1 {
		return 0
	}

	sumF := ToFloat64(args[0])
	for i, v := range args {
		if i == 0 {
			continue
		}

		sumF /= ToFloat64(v)
	}

	return sumF
}

func tmplSqrt(arg interface{}) float64 {
	switch arg.(type) {
	case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
		return math.Sqrt(ToFloat64(arg))
	default:
		return math.Sqrt(-1)
	}
}

func tmplCbrt(arg interface{}) float64 {
	switch arg.(type) {
	case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
		return math.Cbrt(ToFloat64(arg))
	default:
		return math.NaN()
	}
}

func tmplPow(argX, argY interface{}) float64 {
	var xyValue float64
	var xySlice []float64

	switchSlice := []interface{}{argX, argY}

	for _, v := range switchSlice {
		switch v.(type) {
		case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
			xyValue = ToFloat64(v)
		default:
			xyValue = math.NaN()
		}
		xySlice = append(xySlice, xyValue)
	}
	return math.Pow(xySlice[0], xySlice[1])
}

func tmplMax(argX, argY interface{}) float64 {
	var xyValue float64
	var xySlice []float64

	switchSlice := []interface{}{argX, argY}

	for _, v := range switchSlice {
		switch v.(type) {
		case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
			xyValue = ToFloat64(v)
		default:
			xyValue = math.NaN()
		}
		xySlice = append(xySlice, xyValue)
	}
	return math.Max(xySlice[0], xySlice[1])
}

func tmplMin(argX, argY interface{}) float64 {
	var xyValue float64
	var xySlice []float64

	switchSlice := []interface{}{argX, argY}

	for _, v := range switchSlice {
		switch v.(type) {
		case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
			xyValue = ToFloat64(v)
		default:
			xyValue = math.NaN()
		}
		xySlice = append(xySlice, xyValue)
	}
	return math.Min(xySlice[0], xySlice[1])
}

/*
tmplLog is a function for templates using (log base of x = logarithm) as return value.
It is using natural logarithm as default to change the base.
*/
func tmplLog(arguments ...interface{}) (float64, error) {
	var x, base, logarithm float64

	x = ToFloat64(arguments[0])

	if len(arguments) < 1 || len(arguments) > 2 {
		return 0, errors.New("wrong number of arguments")
	} else if len(arguments) == 1 {
		base = math.E
	} else {
		base = ToFloat64(arguments[1])
	}
	/*In an exponential function, the base is always defined to be positive,
	but can't be equal to 1. Because of that also x can't be a negative.*/
	if base == 1 || base <= 0 {
		logarithm = math.NaN()
	} else if base == math.E {
		logarithm = math.Log(x)
	} else {
		logarithm = math.Log(x) / math.Log(base)
	}

	return logarithm, nil
}

func tmplBitwiseAnd(arg0 interface{}, args ...interface{}) int {
	res := tmplToInt(arg0)
	for _, arg := range args {
		res &= tmplToInt(arg)
	}
	return res
}

func tmplBitwiseOr(args ...interface{}) (res int) {
	for _, arg := range args {
		res |= tmplToInt(arg)
	}
	return
}

func tmplBitwiseXor(arg1, arg2 interface{}) int {
	return tmplToInt(arg1) ^ tmplToInt(arg2)
}

func tmplBitwiseNot(arg interface{}) int {
	return ^tmplToInt(arg)
}

func tmplBitwiseAndNot(arg1, arg2 interface{}) int {
	return tmplToInt(arg1) &^ tmplToInt(arg2)
}

func tmplBitwiseLeftShift(arg1, arg2 interface{}) int {
	return tmplToInt(arg1) << tmplToInt(arg2)
}

func tmplBitwiseRightShift(arg1, arg2 interface{}) int {
	return tmplToInt(arg1) >> tmplToInt(arg2)
}

// tmplHumanizeThousands comma separates thousands
func tmplHumanizeThousands(input interface{}) string {
	var f1, f2 string

	i := tmplToInt(input)
	if i < 0 {
		i = i * -1
		f2 = "-"
	}
	str := strconv.Itoa(i)

	idx := 0
	for i = len(str) - 1; i >= 0; i-- {
		idx++
		if idx == 4 {
			idx = 1
			f1 = f1 + ","
		}
		f1 = f1 + string(str[i])
	}

	for i = len(f1) - 1; i >= 0; i-- {
		f2 = f2 + string(f1[i])
	}
	return f2
}

func roleIsAbove(a, b *discordgo.Role) bool {
	return common.IsRoleAbove(a, b)
}

func randInt(args ...interface{}) (int, error) {
	min := int64(0)
	max := int64(10)
	if len(args) >= 2 {
		min = ToInt64(args[0])
		max = ToInt64(args[1])
	} else if len(args) == 1 {
		max = ToInt64(args[0])
	}

	diff := max - min
	if diff <= 0 {
		return 0, errors.New("start must be strictly less than stop")
	}

	r := rand.Int63n(diff)
	return int(r + min), nil
}

func tmplRound(args ...interface{}) float64 {
	if len(args) < 1 {
		return 0
	}
	return math.Round(ToFloat64(args[0]))
}

func tmplRoundCeil(args ...interface{}) float64 {
	if len(args) < 1 {
		return 0
	}
	return math.Ceil(ToFloat64(args[0]))
}

func tmplRoundFloor(args ...interface{}) float64 {
	if len(args) < 1 {
		return 0
	}
	return math.Floor(ToFloat64(args[0]))
}

func tmplRoundEven(args ...interface{}) float64 {
	if len(args) < 1 {
		return 0
	}
	return math.RoundToEven(ToFloat64(args[0]))
}

var ErrStringTooLong = errors.NewPlain("String is too long (max 1MB)")

const MaxStringLength = 1000000

func joinStrings(sep string, args ...interface{}) (string, error) {

	var builder strings.Builder

	for _, v := range args {
		if builder.Len() != 0 {
			builder.WriteString(sep)
		}

		switch t := v.(type) {

		case string:
			builder.WriteString(t)

		case int, uint, int32, uint32, int64, uint64:
			builder.WriteString(ToString(v))

		case float64:
			builder.WriteString(fmt.Sprintf("%g", v))

		case fmt.Stringer:
			builder.WriteString(t.String())

		default:
			cast, ok := castToStringSlice(reflect.ValueOf(v))
			if !ok {
				break
			}

			for j, s := range cast {
				if j != 0 {
					builder.WriteString(sep)
				}

				builder.WriteString(s)
				if builder.Len() > MaxStringLength {
					return "", ErrStringTooLong
				}
			}
		}

		if builder.Len() > MaxStringLength {
			return "", ErrStringTooLong
		}

	}

	return builder.String(), nil
}

var stringSliceType = reflect.TypeOf([]string(nil))

func castToStringSlice(rv reflect.Value) ([]string, bool) {
	rv, _ = indirect(rv)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		// ok
	default:
		return nil, false
	}

	// fast path
	if rv.Type() == stringSliceType {
		return rv.Interface().([]string), true
	}

	ret := make([]string, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		irv, _ := indirect(rv.Index(i))
		if irv.Kind() != reflect.String {
			return nil, false
		}
		ret[i] = irv.String()
	}
	return ret, true
}

func sequence(start, stop int) ([]int, error) {

	if stop < start {
		return nil, errors.New("stop is less than start?")
	}

	if stop-start > MaxSliceLength {
		return nil, fmt.Errorf("sequence max length is %d", MaxSliceLength)
	}

	out := make([]int, stop-start)

	ri := 0
	for i := start; i < stop; i++ {
		out[ri] = i
		ri++
	}
	return out, nil
}

// shuffle returns the given rangeable list in a randomised order.
func shuffle(seq interface{}) (interface{}, error) {
	if seq == nil {
		return nil, errors.New("both count and seq must be provided")
	}

	seqv := reflect.ValueOf(seq)
	seqv, isNil := indirect(seqv)
	if isNil {
		return nil, errors.New("can't iterate over a nil value")
	}

	if seqv.Kind() != reflect.Slice {
		return nil, errors.New("can't iterate over " + reflect.ValueOf(seq).Type().String())
	}

	shuffled := reflect.MakeSlice(seqv.Type(), seqv.Len(), seqv.Len())

	rand.Seed(time.Now().UTC().UnixNano())
	randomIndices := rand.Perm(seqv.Len())

	for index, value := range randomIndices {
		shuffled.Index(value).Set(seqv.Index(index))
	}

	return shuffled.Interface(), nil
}

func tmplToInt(from any) int {
	t := reflect.ValueOf(from)
	switch {
	case t.CanInt():
		return int(t.Int())
	case t.CanFloat():
		return int(t.Float())
	case t.CanUint():
		return int(t.Uint())
	case t.Kind() == reflect.String:
		// base 0 to infer hex, octal, etc. from string prefix
		parsed, _ := strconv.ParseInt(t.String(), 0, 64)
		return int(parsed)
	default:
		return 0
	}
}

func ToInt64(from any) int64 {
	t := reflect.ValueOf(from)
	switch {
	case t.CanInt():
		return t.Int()
	case t.CanFloat():
		return int64(t.Float())
	case t.CanUint():
		return int64(t.Uint())
	case t.Kind() == reflect.String:
		parsed, _ := strconv.ParseInt(t.String(), 0, 64)
		return parsed
	default:
		return 0
	}
}

func ToString(from interface{}) string {
	switch t := from.(type) {
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'E', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'E', -1, 64)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(uint64(t), 10)
	case []rune:
		return string(t)
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	case string:
		return t
	default:
		return ""
	}
}

func ToFloat64(from interface{}) float64 {
	t := reflect.ValueOf(from)
	switch {
	case t.CanInt():
		return float64(t.Int())
	case t.CanFloat():
		return t.Float()
	case t.CanUint():
		return float64(t.Uint())
	case t.Kind() == reflect.String:
		parsed, _ := strconv.ParseFloat(t.String(), 64)
		return parsed
	default:
		return 0
	}
}

func ToDuration(from interface{}) time.Duration {
	switch t := from.(type) {
	case int, int32, int64, float32, float64, uint, uint32, uint64:
		return time.Duration(ToInt64(t))
	case string:
		parsed, err := common.ParseDuration(t)
		if parsed < time.Second || err != nil {
			return 0
		}
		return parsed
	case time.Duration:
		return t
	default:
		return 0
	}
}

func ToRune(from interface{}) []rune {
	switch t := from.(type) {
	case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
		return []rune(ToString(t))
	case string:
		return []rune(t)
	default:
		return nil
	}
}

func ToByte(from interface{}) []byte {
	switch t := from.(type) {
	case int, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
		return []byte(ToString(t))
	case string:
		return []byte(t)
	default:
		return nil
	}
}

func tmplJson(v interface{}, flags ...bool) (string, error) {
	var b []byte
	var err error

	switch len(flags) {

	case 0:
		b, err = json.Marshal(v)
		if err != nil {
			return "", err
		}

	case 1:
		if flags[0] {
			b, err = json.MarshalIndent(v, "", "\t")
			if err != nil {
				return "", err
			}
		} else {
			b, err = json.Marshal(v)
			if err != nil {
				return "", err
			}
		}

	default:
		return "", errors.New("Too many flags")
	}

	return string(b), nil
}

func tmplJSONToSDict(v interface{}) (SDict, error) {
	var toSDict SDict
	err := json.Unmarshal([]byte(ToString(v)), &toSDict)
	if err != nil {
		return nil, err
	}

	return toSDict, nil
}

func tmplFormatTime(t time.Time, args ...string) string {
	layout := time.RFC822
	if len(args) > 0 {
		layout = args[0]
	}

	return t.Format(layout)
}

func tmplSnowflakeToTime(v interface{}) time.Time {
	return bot.SnowflakeToTime(ToInt64(v)).UTC()
}

func tmplTimestampToTime(v interface{}) time.Time {
	return time.Unix(ToInt64(v), 0).UTC()
}

type variadicFunc func([]reflect.Value) (reflect.Value, error)

// callVariadic allows the given function to be called with either a variadic
// sequence of arguments (i.e., fixed in the template definition) or a slice
// (i.e., from a pipeline or context variable). In effect, a limited `flatten`
// operation.
func callVariadic(f variadicFunc, skipNil bool, values ...reflect.Value) (reflect.Value, error) {
	var vs []reflect.Value
	for _, val := range values {
		v, _ := indirect(val)
		switch {
		case !v.IsValid():
			if !skipNil {
				vs = append(vs, v)
			} else {
				continue
			}
		case v.Kind() == reflect.Array || v.Kind() == reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				irv, _ := indirect(v.Index(i))
				vs = append(vs, irv)
			}
		default:
			vs = append(vs, v)
		}
	}

	return f(vs)
}

// slice returns the result of creating a new slice with the given arguments.
// "slice x 1 2" is, in Go syntax, x[1:2], and "slice x 1" is equivalent to
// x[1:].
func slice(item reflect.Value, indices ...reflect.Value) (reflect.Value, error) {
	v, _ := indirect(item)
	if !v.IsValid() {
		return reflect.Value{}, errors.New("index of untyped nil")
	}

	var args []int
	for _, i := range indices {
		index, _ := indirect(i)
		switch index.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			args = append(args, int(index.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			args = append(args, int(index.Uint()))
		case reflect.Invalid:
			return reflect.Value{}, errors.New("cannot index slice/array with nil")
		default:
			return reflect.Value{}, errors.Errorf("cannot index slice/array with type %s", index.Type())
		}
	}

	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		startIndex := 0
		endIndex := 0

		switch len(args) {
		case 0:
			// No start or end index provided same as slice[:]
			return v, nil
		case 1:
			// Only start index provided, same as slice[i:]
			startIndex = args[0]
			endIndex = v.Len()
			// args = append(args, v.Len()+1-args[0])
		case 2:
			// Both start and end index provided
			startIndex = args[0]
			endIndex = args[1]
		default:
			return reflect.Value{}, errors.Errorf("unexpected slice arguments %d", len(args))
		}

		if startIndex < 0 || startIndex >= v.Len() {
			return reflect.Value{}, errors.Errorf("start index out of range: %d", startIndex)
		} else if endIndex <= startIndex || endIndex > v.Len() {
			return reflect.Value{}, errors.Errorf("end index out of range: %d", endIndex)
		}

		return v.Slice(startIndex, endIndex), nil
	default:
		return reflect.Value{}, errors.Errorf("can't index item of type %s", v.Type())
	}
}

func tmplCurrentTime() time.Time {
	return time.Now().UTC()
}

func tmplParseTime(input string, layout interface{}, locations ...string) (time.Time, error) {
	loc := time.UTC

	var err error
	if len(locations) > 0 {
		loc, err = time.LoadLocation(locations[0])
		if err != nil {
			return time.Time{}, err
		}
	}

	var parsed time.Time

	rv, _ := indirect(reflect.ValueOf(layout))
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() > 50 {
			return time.Time{}, errors.New("max number of layouts is 50")
		}

		for i := 0; i < rv.Len(); i++ {
			lv, _ := indirect(rv.Index(i))
			if lv.Kind() != reflect.String {
				return time.Time{}, errors.New("layout must be either a slice of strings or a single string")
			}

			parsed, err = time.ParseInLocation(lv.String(), input, loc)
			if err == nil {
				// found a layout that matched
				break
			}
		}
	case reflect.String:
		parsed, _ = time.ParseInLocation(rv.String(), input, loc)
	default:
		return time.Time{}, errors.New("layout must be either a slice of strings or a single string")
	}

	// if no layout matched, parsed will be the zero Time.
	// thus, users can call <time>.IsZero() to determine whether parseTime() was able to parse the time.
	return parsed, nil
}

func tmplNewDate(year, monthInt, day, hour, min, sec int, location ...string) (time.Time, error) {
	loc := time.UTC
	month := time.Month(monthInt)

	var err error
	if len(location) >= 1 {
		loc, err = time.LoadLocation(location[0])
		if err != nil {
			return time.Time{}, err
		}
	}

	return time.Date(year, month, day, hour, min, sec, 0, loc), nil
}

func tmplWeekNumber(t time.Time) (week int) {
	_, week = t.ISOWeek()
	return
}

func tmplHumanizeDurationHours(in interface{}) string {
	return common.HumanizeDuration(common.DurationPrecisionHours, ToDuration(in))
}

func tmplHumanizeDurationMinutes(in interface{}) string {
	return common.HumanizeDuration(common.DurationPrecisionMinutes, ToDuration(in))
}

func tmplHumanizeDurationSeconds(in interface{}) string {
	return common.HumanizeDuration(common.DurationPrecisionSeconds, ToDuration(in))
}

func tmplHumanizeTimeSinceDays(in time.Time) string {
	return common.HumanizeDuration(common.DurationPrecisionDays, time.Since(in))
}
