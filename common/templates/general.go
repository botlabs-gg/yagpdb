package templates

import (
	"encoding/json"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// dictionary creates a map[string]interface{} from the given parameters by
// walking the parameters and treating them as key-value pairs.  The number
// of parameters must be even.
func Dictionary(values ...interface{}) (map[interface{}]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("invalid dict call")
	}
	dict := make(map[interface{}]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key := values[i]
		dict[key] = values[i+1]
	}

	return dict, nil
}

func StringKeyDictionary(values ...interface{}) (map[string]interface{}, error) {
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

	return dict, nil
}

func CreateSlice(values ...interface{}) ([]interface{}, error) {
	slice := make([]interface{}, len(values))
	for i := 0; i < len(values); i++ {
		slice[i] = values[i]
	}

	return slice, nil
}

func CreateEmbed(values ...interface{}) (*discordgo.MessageEmbed, error) {
	dict, err := StringKeyDictionary(values...)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(dict)
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

func add(x, y int) int {
	return x + y
}

func roleIsAbove(a, b *discordgo.Role) bool {
	return dutil.IsRoleAbove(a, b)
}

func randInt(args ...interface{}) int {
	min := int64(0)
	max := int64(10)
	if len(args) >= 2 {
		min = ToInt64(args[0])
		max = ToInt64(args[1])
	} else if len(args) == 1 {
		max = ToInt64(args[0])
	}

	r := rand.Int63n(max - min)
	return int(r + min)
}

func joinStrings(sep string, args ...interface{}) string {

	out := ""

	for _, v := range args {
		switch t := v.(type) {
		case string:
			if out != "" {
				out += sep
			}

			out += t
		case []string:
			for _, s := range t {
				if out != "" {
					out += sep
				}

				out += s
			}
		case int, int32, uint32, int64, uint64:
			out += ToString(v)
		}
	}

	return out
}

func sequence(start, stop int) ([]int, error) {

	if stop-start > 1000 {
		return nil, errors.New("Sequence max length is 1000")
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

	switch seqv.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		// okay
	default:
		return nil, errors.New("can't iterate over " + reflect.ValueOf(seq).Type().String())
	}

	shuffled := reflect.MakeSlice(reflect.TypeOf(seq), seqv.Len(), seqv.Len())

	rand.Seed(time.Now().UTC().UnixNano())
	randomIndices := rand.Perm(seqv.Len())

	for index, value := range randomIndices {
		shuffled.Index(value).Set(seqv.Index(index))
	}

	return shuffled.Interface(), nil
}

func str(arg interface{}) string {
	switch v := arg.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	}

	return ""
}

func tmplToInt(from interface{}) int {
	switch t := from.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float32:
		return int(t)
	case float64:
		return int(t)
	case string:
		parsed, _ := strconv.ParseInt(t, 10, 64)
		return int(parsed)
	default:
		return 0
	}
}

func ToInt64(from interface{}) int64 {
	switch t := from.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		parsed, _ := strconv.ParseInt(t, 10, 64)
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
	case string:
		return t
	default:
		return ""
	}
}

func tmplJson(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func tmplFormatTime(t time.Time, args ...string) string {
	layout := time.RFC822
	if len(args) > 0 {
		layout = args[0]
	}

	return t.Format(layout)
}

type variadicFunc func([]reflect.Value) (reflect.Value, error)

// callVariadic allows the given function to be called with either a variadic
// sequence of arguments (i.e., fixed in the template definition) or a slice
// (i.e., from a pipeline or context variable). In effect, a limited `flatten`
// operation.
func callVariadic(f variadicFunc, values ...reflect.Value) (reflect.Value, error) {
	var vs []reflect.Value
	for _, val := range values {
		v, _ := indirect(val)
		switch {
		case !v.IsValid():
			continue
		case v.Kind() == reflect.Array || v.Kind() == reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				vs = append(vs, v.Index(i))
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
			break
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
	return time.Now()
}

func tmplEscapeHere(in string) string {
	return common.EscapeEveryoneHere(in, false, true)
}
func tmplEscapeEveryone(in string) string {
	return common.EscapeEveryoneHere(in, true, false)
}
func tmplEscapeEveryoneHere(in string) string {
	return common.EscapeEveryoneHere(in, true, true)
}

func tmplHumanizeDurationHours(in time.Duration) string {
	return common.HumanizeDuration(common.DurationPrecisionHours, in)
}

func tmplHumanizeTimeSinceDays(in time.Time) string {
	return common.HumanizeDuration(common.DurationPrecisionDays, time.Since(in))
}
