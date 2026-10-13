package templates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/vmihailenco/msgpack"
)

const maxStoredValueDepth = 64

var errStoredValueTooDeep = errors.New("componentBuilder value is nested too deeply to store")

// storedComponent carries a discord component through msgpack as its json,
// since plain msgpack encoding drops the component type.
type storedComponent struct {
	JSON []byte
}

func (c *storedComponent) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeBytes(c.JSON)
}

func (c *storedComponent) DecodeMsgpack(dec *msgpack.Decoder) (err error) {
	c.JSON, err = dec.DecodeBytes()
	return
}

func (s *ComponentBuilder) EncodeMsgpack(enc *msgpack.Encoder) error {
	if err := enc.EncodeArrayLen(len(s.Components)); err != nil {
		return err
	}

	for i, key := range s.Components {
		value, err := toStorableValue(s.Values[i], 0)
		if err != nil {
			return err
		}

		if err := enc.EncodeString(key); err != nil {
			return err
		}
		if err := enc.Encode(value); err != nil {
			return err
		}
	}

	return nil
}

func (s *ComponentBuilder) DecodeMsgpack(dec *msgpack.Decoder) error {
	n, err := dec.DecodeArrayLen()
	if err != nil {
		return err
	}

	for range n {
		key, err := dec.DecodeString()
		if err != nil {
			return err
		}

		value, err := dec.DecodeInterface()
		if err != nil {
			return err
		}
		value, err = fromStorableValue(value, 0)
		if err != nil {
			return err
		}

		if _, err := s.Add(key, value); err != nil {
			return err
		}
	}

	return nil
}

// toStorableValue returns a copy of value with every discord component
// replaced by a storedComponent.
func toStorableValue(value any, depth int) (any, error) {
	if depth > maxStoredValueDepth {
		return nil, errStoredValueTooDeep
	}
	depth++

	switch typed := value.(type) {
	case nil, *ComponentBuilder:
		return value, nil
	case discordgo.MessageComponent:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, err
		}
		return &storedComponent{JSON: encoded}, nil
	case *SDict:
		return toStorableValue(*typed, depth)
	case *Dict:
		return toStorableValue(*typed, depth)
	case *Slice:
		return toStorableValue(*typed, depth)
	case SDict:
		converted := make(SDict, len(typed))
		for k, v := range typed {
			storable, err := toStorableValue(v, depth)
			if err != nil {
				return nil, err
			}
			converted[k] = storable
		}
		return converted, nil
	case Dict:
		converted := make(Dict, len(typed))
		for k, v := range typed {
			storable, err := toStorableValue(v, depth)
			if err != nil {
				return nil, err
			}
			converted[k] = storable
		}
		return converted, nil
	case map[string]any:
		converted := make(map[string]any, len(typed))
		for k, v := range typed {
			storable, err := toStorableValue(v, depth)
			if err != nil {
				return nil, err
			}
			converted[k] = storable
		}
		return converted, nil
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return value, nil
	}
	if isPlainElem(val.Type().Elem().Kind()) {
		return value, nil
	}

	converted := make(Slice, val.Len())
	for i := range val.Len() {
		storable, err := toStorableValue(val.Index(i).Interface(), depth)
		if err != nil {
			return nil, err
		}
		converted[i] = storable
	}
	return converted, nil
}

func isPlainElem(kind reflect.Kind) bool {
	switch kind {
	case reflect.Interface, reflect.Ptr, reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return false
	}
	return true
}

// fromStorableValue restores the discord components in a decoded value in
// place.
func fromStorableValue(value any, depth int) (any, error) {
	if depth > maxStoredValueDepth {
		return nil, errStoredValueTooDeep
	}
	depth++

	switch typed := value.(type) {
	case *storedComponent:
		return discordgo.MessageComponentFromJSON(typed.JSON)
	case *SDict:
		return typed, restoreMapValues(*typed, depth)
	case SDict:
		return typed, restoreMapValues(typed, depth)
	case map[string]any:
		return typed, restoreMapValues(typed, depth)
	case *Dict:
		return typed, restoreMapValues(*typed, depth)
	case Dict:
		return typed, restoreMapValues(typed, depth)
	case map[any]any:
		return typed, restoreMapValues(typed, depth)
	case *Slice:
		return typed, restoreSliceValues(*typed, depth)
	case Slice:
		return typed, restoreSliceValues(typed, depth)
	case []any:
		return typed, restoreSliceValues(typed, depth)
	}
	return value, nil
}

func restoreMapValues[K comparable, M ~map[K]any](m M, depth int) error {
	for k, v := range m {
		restored, err := fromStorableValue(v, depth)
		if err != nil {
			return err
		}
		m[k] = restored
	}
	return nil
}

func restoreSliceValues[S ~[]any](s S, depth int) error {
	for i, v := range s {
		restored, err := fromStorableValue(v, depth)
		if err != nil {
			return err
		}
		s[i] = restored
	}
	return nil
}

// componentBuilderFromMessage recreates a builder from a components v2
// message. File components are dropped as they reference attachments of the
// original message, which can't be reused.
func componentBuilderFromMessage(msg *discordgo.Message) (*ComponentBuilder, error) {
	if msg.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 {
		return nil, errors.New("message is not a components v2 message")
	}

	// a json round trip deep copies the components, so later custom id
	// validation can't mutate the original message
	encoded, err := json.Marshal(withoutFileComponents(msg.Components))
	if err != nil {
		return nil, err
	}

	var rawComponents []json.RawMessage
	if err := json.Unmarshal(encoded, &rawComponents); err != nil {
		return nil, err
	}

	builder := &ComponentBuilder{}
	for _, raw := range rawComponents {
		component, err := discordgo.MessageComponentFromJSON(raw)
		if err != nil {
			return nil, err
		}

		key, err := componentBuilderKey(component)
		if err != nil {
			return nil, err
		}
		if _, err := builder.Add(key, component); err != nil {
			return nil, err
		}
	}

	if msg.Flags&discordgo.MessageFlagsSuppressEmbeds != 0 {
		builder.Add("suppress_embeds", true)
	}
	if msg.Flags&discordgo.MessageFlagsSuppressNotifications != 0 {
		builder.Add("silent", true)
	}

	return builder, nil
}

func withoutFileComponents(components []discordgo.TopLevelComponent) []discordgo.TopLevelComponent {
	result := make([]discordgo.TopLevelComponent, 0, len(components))
	for _, component := range components {
		switch typed := component.(type) {
		case *discordgo.ComponentFile:
			continue
		case *discordgo.Container:
			container := *typed
			container.Components = withoutFileComponents(typed.Components)
			component = &container
		}
		result = append(result, component)
	}
	return result
}

func componentBuilderKey(component discordgo.MessageComponent) (string, error) {
	switch component.(type) {
	case *discordgo.ActionsRow:
		return "interactive_components", nil
	case *discordgo.Section:
		return "section", nil
	case *discordgo.TextDisplay:
		return "text", nil
	case *discordgo.MediaGallery:
		return "gallery", nil
	case *discordgo.Separator:
		return "separator", nil
	case *discordgo.Container:
		return "container", nil
	}
	return "", fmt.Errorf("unsupported component type %d in componentBuilder", component.Type())
}

// Builders stored before ComponentBuilder had its own msgpack encoding were
// saved as plain {Components, Values} maps, with any discord component
// flattened into a map keyed by its go field names and no component type.

var legacyComponentTypes = []reflect.Type{
	reflect.TypeOf(discordgo.ActionsRow{}),
	reflect.TypeOf(discordgo.Button{}),
	reflect.TypeOf(discordgo.SelectMenu{}),
	reflect.TypeOf(discordgo.TextInput{}),
	reflect.TypeOf(discordgo.Section{}),
	reflect.TypeOf(discordgo.TextDisplay{}),
	reflect.TypeOf(discordgo.Thumbnail{}),
	reflect.TypeOf(discordgo.MediaGallery{}),
	reflect.TypeOf(discordgo.ComponentFile{}),
	reflect.TypeOf(discordgo.Separator{}),
	reflect.TypeOf(discordgo.Container{}),
	reflect.TypeOf(discordgo.Label{}),
	reflect.TypeOf(discordgo.RadioGroup{}),
	reflect.TypeOf(discordgo.CheckboxGroup{}),
	reflect.TypeOf(discordgo.Checkbox{}),
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case SDict:
		return typed, true
	case *SDict:
		return *typed, true
	}
	return nil, false
}

func isLegacyComponentBuilder(m map[string]any) bool {
	if len(m) != 2 {
		return false
	}
	_, hasComponents := m["Components"]
	_, hasValues := m["Values"]
	return hasComponents && hasValues
}

func componentBuilderFromLegacy(m map[string]any, depth int) (*ComponentBuilder, error) {
	keys, err := legacySlice(m["Components"])
	if err != nil {
		return nil, err
	}
	values, err := legacySlice(m["Values"])
	if err != nil {
		return nil, err
	}
	if len(keys) != len(values) {
		return nil, errors.New("stored componentBuilder has mismatched keys and values")
	}

	builder := &ComponentBuilder{}
	for i, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			return nil, errors.New("stored componentBuilder has a non string key")
		}

		value, err := fromLegacyValue(values[i], depth)
		if err != nil {
			return nil, err
		}
		if _, err := builder.Add(keyStr, value); err != nil {
			return nil, err
		}
	}
	return builder, nil
}

func legacySlice(value any) ([]any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []any:
		return typed, nil
	case Slice:
		return typed, nil
	case *Slice:
		return *typed, nil
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice {
		return nil, errors.New("stored componentBuilder is malformed")
	}
	result := make([]any, val.Len())
	for i := range val.Len() {
		result[i] = val.Index(i).Interface()
	}
	return result, nil
}

func fromLegacyValue(value any, depth int) (any, error) {
	if depth > maxStoredValueDepth {
		return nil, errStoredValueTooDeep
	}
	depth++

	switch typed := value.(type) {
	case map[string]any:
		if isLegacyComponentBuilder(typed) {
			return componentBuilderFromLegacy(typed, depth)
		}
		if typ, ok := matchLegacyComponentType(typed); ok {
			return legacyComponent(typ, typed, depth)
		}
		return typed, restoreLegacyMapValues(typed, depth)
	case *SDict:
		return typed, restoreLegacyMapValues(*typed, depth)
	case *Dict:
		return typed, restoreLegacyMapValues(*typed, depth)
	case map[any]any:
		return typed, restoreLegacyMapValues(typed, depth)
	case *Slice:
		return typed, restoreLegacySliceValues(*typed, depth)
	case []any:
		return typed, restoreLegacySliceValues(typed, depth)
	}
	return value, nil
}

func restoreLegacyMapValues[K comparable, M ~map[K]any](m M, depth int) error {
	for k, v := range m {
		restored, err := fromLegacyValue(v, depth)
		if err != nil {
			return err
		}
		m[k] = restored
	}
	return nil
}

func restoreLegacySliceValues[S ~[]any](s S, depth int) error {
	for i, v := range s {
		restored, err := fromLegacyValue(v, depth)
		if err != nil {
			return err
		}
		s[i] = restored
	}
	return nil
}

// matchLegacyComponentType picks the component type whose exported fields
// cover all keys of m with the fewest fields left over, so components saved
// before a field was added to discordgo still match.
func matchLegacyComponentType(m map[string]any) (reflect.Type, bool) {
	if len(m) == 0 {
		return nil, false
	}

	var best reflect.Type
	bestExtra := -1
	for _, typ := range legacyComponentTypes {
		fields := exportedFieldNames(typ)
		covered := true
		for key := range m {
			if !fields[key] {
				covered = false
				break
			}
		}
		if !covered {
			continue
		}

		extra := len(fields) - len(m)
		if bestExtra == -1 || extra < bestExtra {
			best, bestExtra = typ, extra
		}
	}
	return best, best != nil
}

func exportedFieldNames(typ reflect.Type) map[string]bool {
	names := make(map[string]bool, typ.NumField())
	for i := range typ.NumField() {
		if f := typ.Field(i); f.IsExported() {
			names[f.Name] = true
		}
	}
	return names
}

func legacyComponent(typ reflect.Type, m map[string]any, depth int) (discordgo.MessageComponent, error) {
	converted, err := legacyToJSONValue(typ, m, depth)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(converted)
	if err != nil {
		return nil, err
	}

	component := reflect.New(typ)
	if err := json.Unmarshal(encoded, component.Interface()); err != nil {
		return nil, fmt.Errorf("failed recovering stored %s: %w", typ.Name(), err)
	}
	return component.Interface().(discordgo.MessageComponent), nil
}

// legacyToJSONValue converts a value saved under go field names into the
// shape typ expects in json.
func legacyToJSONValue(typ reflect.Type, value any, depth int) (any, error) {
	if depth > maxStoredValueDepth {
		return nil, errStoredValueTooDeep
	}
	depth++

	if value == nil {
		return nil, nil
	}

	switch typ.Kind() {
	case reflect.Ptr:
		return legacyToJSONValue(typ.Elem(), value, depth)
	case reflect.Interface:
		m, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("stored component is malformed")
		}
		componentType, ok := matchLegacyComponentType(m)
		if !ok {
			return nil, errors.New("stored component has an unknown type")
		}
		return legacyComponent(componentType, m, depth)
	case reflect.Slice, reflect.Array:
		items, err := legacySlice(value)
		if err != nil {
			return nil, err
		}
		converted := make([]any, len(items))
		for i, item := range items {
			if converted[i], err = legacyToJSONValue(typ.Elem(), item, depth); err != nil {
				return nil, err
			}
		}
		return converted, nil
	case reflect.Struct:
		m, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("stored component is malformed")
		}
		converted := make(map[string]any, len(m))
		for i := range typ.NumField() {
			field := typ.Field(i)
			fieldValue, present := m[field.Name]
			if !field.IsExported() || !present {
				continue
			}

			name, opts, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}

			jsonValue, err := legacyToJSONValue(field.Type, fieldValue, depth)
			if err != nil {
				return nil, err
			}
			if jsonValue != nil && strings.Contains(opts, "string") {
				jsonValue = fmt.Sprint(jsonValue)
			}
			converted[name] = jsonValue
		}
		return converted, nil
	}
	return value, nil
}
