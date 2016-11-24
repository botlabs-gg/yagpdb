package web

import (
	"bytes"
	"errors"
	"github.com/jonas747/discordgo"
	"html/template"
	"reflect"
	"strings"
)

// mTemplate combines "template" with dictionary. so you can specify multiple variables
// and call templates almost as if they were functions with arguments
// makes certain templates a lot simpler
func mTemplate(name string, values ...interface{}) (template.HTML, error) {

	data, err := dictionary(values...)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = Templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil
}

// dictionary creates a map[string]interface{} from the given parameters by
// walking the parameters and treating them as key-value pairs.  The number
// of parameters must be even.
func dictionary(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
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
	lv := reflect.ValueOf(l)
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
				if vv.Type() == lvv.Type() && vv.String() == lvv.String() {
					return true
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch vv.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if vv.Int() == lvv.Int() {
						return true
					}
				}
			case reflect.Float32, reflect.Float64:
				switch vv.Kind() {
				case reflect.Float32, reflect.Float64:
					if vv.Float() == lvv.Float() {
						return true
					}
				}
			}
		}
	case reflect.String:
		if vv.Type() == lv.Type() && strings.Contains(lv.String(), vv.String()) {
			return true
		}
	}

	return false
}

var permsString = map[string]int{
	"ManageRoles":    discordgo.PermissionManageRoles,
	"ManageMessages": discordgo.PermissionManageMessages,
}

func hasPerm(botPerms int, checkPerm string) (bool, error) {
	p, ok := permsString[checkPerm]
	if !ok {
		return false, errors.New("Unknown permission: " + checkPerm)
	}

	return botPerms&p != 0, nil
}
