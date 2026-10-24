package templates

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/vmihailenco/msgpack"
)

type testComponents struct {
	button, linkButton, sectionButton *discordgo.Button
	menu                              *discordgo.SelectMenu
}

func newTestComponents(t *testing.T) testComponents {
	t.Helper()
	button, err := CreateButton(SDict{"label": "Click", "custom_id": "click", "emoji": SDict{"id": "123456789012345678", "name": "pepe"}})
	if err != nil {
		t.Fatal(err)
	}
	linkButton, err := CreateButton(SDict{"label": "Docs", "style": "link", "url": "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	sectionButton, err := CreateButton(SDict{"label": "s", "custom_id": "sec"})
	if err != nil {
		t.Fatal(err)
	}
	menu, err := CreateSelectMenu(SDict{"custom_id": "menu", "min_values": 0, "options": Slice{SDict{"label": "a", "value": "a"}, SDict{"label": "b", "value": "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	return testComponents{button, linkButton, sectionButton, menu}
}

// newTestComponentBuilder mixes plain data, typed components at several
// depths, nested builders and a file next to a typed component.
func newTestComponentBuilder(t *testing.T) *ComponentBuilder {
	t.Helper()
	c := newTestComponents(t)

	builder, err := CreateComponentBuilder(
		"text", "hello",
		"text", Slice{"one", "two"},
		"separator", true,
		"container", SDict{
			"color": 0xff0000,
			"components": &ComponentBuilder{
				Components: []string{"text", "buttons", "section", "file"},
				Values: []any{
					"inside",
					Slice{c.button, c.linkButton},
					SDict{"text": "section text", "button": c.sectionButton},
					SDict{"content": "file body", "name": "notes"},
				},
			},
		},
		"gallery", Slice{SDict{"media": "https://example.com/a.png"}},
		"buttons", []*discordgo.Button{c.button},
		"menus", c.menu,
		"silent", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	return builder
}

func messageJSON(t *testing.T, builder *ComponentBuilder) string {
	t.Helper()
	msg, err := builder.ToComplexMessage()
	if err != nil {
		t.Fatal(err)
	}

	var files []string
	for _, f := range msg.Files {
		files = append(files, f.Name)
	}
	encoded, err := json.Marshal(struct {
		Components []discordgo.TopLevelComponent
		Flags      discordgo.MessageFlags
		Files      []string
	}{msg.Components, msg.Flags, files})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func msgpackRoundTrip(t *testing.T, value any) any {
	t.Helper()
	encoded, err := msgpack.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := msgpack.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestComponentBuilderMsgpackRoundTrip(t *testing.T) {
	original := newTestComponentBuilder(t)
	expected := messageJSON(t, original)

	decoded, ok := msgpackRoundTrip(t, original).(*ComponentBuilder)
	if !ok {
		t.Fatal("decoded value is not a *ComponentBuilder")
	}

	if !reflect.DeepEqual(decoded.Components, original.Components) {
		t.Errorf("keys changed: expected %v, got %v", original.Components, decoded.Components)
	}
	if got := messageJSON(t, decoded); got != expected {
		t.Errorf("round trip changed the message\nexpected: %s\ngot:      %s", expected, got)
	}

	buttons := decoded.Get("buttons")
	if len(buttons) != 1 {
		t.Fatalf("expected Get to find the buttons entry, got %v", buttons)
	}
	if button := (*buttons[0].(*Slice))[0]; reflect.TypeOf(button) != reflect.TypeFor[*discordgo.Button]() {
		t.Errorf("expected a typed button, got %T", button)
	}

	again := msgpackRoundTrip(t, decoded).(*ComponentBuilder)
	if got := messageJSON(t, again); got != expected {
		t.Errorf("second round trip changed the message\nexpected: %s\ngot:      %s", expected, got)
	}
}

func TestComponentBuilderMsgpackDoesNotMutate(t *testing.T) {
	original := newTestComponentBuilder(t)
	containerBefore := original.Values[3].(SDict)["components"].(*ComponentBuilder).Values[1].(Slice)[0]

	msgpackRoundTrip(t, original)

	containerAfter := original.Values[3].(SDict)["components"].(*ComponentBuilder).Values[1].(Slice)[0]
	if containerBefore != containerAfter {
		t.Error("storing the builder replaced values in the original")
	}
}

func TestComponentBuilderMsgpackInvalidBuilder(t *testing.T) {
	original, _ := CreateComponentBuilder("not_a_component_yet", SDict{"a": 1})
	decoded := msgpackRoundTrip(t, original).(*ComponentBuilder)

	if !reflect.DeepEqual(decoded.Components, []string{"not_a_component_yet"}) {
		t.Errorf("unexpected keys %v", decoded.Components)
	}
}

func TestComponentBuilderMsgpackTooDeep(t *testing.T) {
	var value any = "leaf"
	for range maxStoredValueDepth + 2 {
		value = Slice{value}
	}
	builder, _ := CreateComponentBuilder("text", value)
	if _, err := msgpack.Marshal(builder); err == nil {
		t.Fatal("expected an error storing an overly nested value")
	}
}

// legacyComponentBuilder mirrors how ComponentBuilder was encoded before it
// had its own msgpack encoding.
type legacyComponentBuilder struct {
	Components []string
	Values     []any
}

func TestComponentBuilderFromLegacy(t *testing.T) {
	c := newTestComponents(t)
	section, err := CreateSection(SDict{"text": "typed section", "thumbnail": SDict{"media": "https://example.com/t.png"}})
	if err != nil {
		t.Fatal(err)
	}

	newBuilder := func(nested any) *ComponentBuilder {
		b, err := CreateComponentBuilder(
			"text", "hello",
			"container", SDict{"color": 0xff0000, "components": nested},
			"section", section,
			"buttons", Slice{c.button, c.linkButton},
			"menus", c.menu,
			"silent", true,
		)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	nestedValues := []any{"inside", Slice{c.sectionButton}}
	expected := messageJSON(t, newBuilder(&ComponentBuilder{Components: []string{"text", "buttons"}, Values: nestedValues}))

	current := newBuilder(nil)
	current.Values[1].(SDict)["components"] = legacyComponentBuilder{Components: []string{"text", "buttons"}, Values: nestedValues}
	legacy := legacyComponentBuilder{Components: current.Components, Values: current.Values}

	stored := msgpackRoundTrip(t, legacy)
	m, ok := stored.(map[string]any)
	if !ok {
		t.Fatalf("legacy value decoded as %T", stored)
	}

	recovered, err := CreateComponentBuilder(m)
	if err != nil {
		t.Fatal(err)
	}
	if got := messageJSON(t, recovered); got != expected {
		t.Errorf("legacy recovery changed the message\nexpected: %s\ngot:      %s", expected, got)
	}
	if !reflect.DeepEqual(recovered.Components, legacy.Components) {
		t.Errorf("keys changed: expected %v, got %v", legacy.Components, recovered.Components)
	}
}

func TestLegacyComponentTypesAreDistinguishable(t *testing.T) {
	seen := map[string]string{}
	for _, typ := range legacyComponentTypes {
		var names []string
		for name := range exportedFieldNames(typ) {
			names = append(names, name)
		}
		sort.Strings(names)
		key := strings.Join(names, ",")
		if other, ok := seen[key]; ok {
			t.Errorf("%s and %s have identical fields", typ.Name(), other)
		}
		seen[key] = typ.Name()

		fields := map[string]any{}
		for _, name := range names {
			fields[name] = nil
		}
		if match, _ := matchLegacyComponentType(fields); match != typ {
			t.Errorf("fields of %s matched %v", typ.Name(), match)
		}
	}
}

func TestComponentBuilderFromMessage(t *testing.T) {
	sent, err := newTestComponentBuilder(t).ToComplexMessage()
	if err != nil {
		t.Fatal(err)
	}

	encodedComponents, err := json.Marshal(sent.Components)
	if err != nil {
		t.Fatal(err)
	}
	flags, _ := json.Marshal(sent.Flags)
	raw := `{"id":"1","channel_id":"2","flags":` + string(flags) + `,"components":` + string(encodedComponents) + `}`

	var msg discordgo.Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatal(err)
	}

	builder, err := CreateComponentBuilder(&msg)
	if err != nil {
		t.Fatal(err)
	}

	resent, err := builder.ToComplexMessage()
	if err != nil {
		t.Fatal(err)
	}

	expected := strings.Replace(string(encodedComponents), `,{"file":{"url":"attachment://notes.txt"},"type":13}`, "", 1)
	if expected == string(encodedComponents) {
		t.Fatal("test message is missing the file component")
	}
	got, _ := json.Marshal(resent.Components)
	if expected != string(got) {
		t.Errorf("message conversion changed components\nexpected: %s\ngot:      %s", expected, got)
	}
	if resent.Flags != sent.Flags {
		t.Errorf("expected flags %d, got %d", sent.Flags, resent.Flags)
	}

	builder.Values[0].(*discordgo.TextDisplay).Content = "changed"
	if msg.Components[0].(*discordgo.TextDisplay).Content == "changed" {
		t.Error("builder shares components with the source message")
	}
}

func TestComponentBuilderFromNonV2Message(t *testing.T) {
	if _, err := CreateComponentBuilder(&discordgo.Message{Content: "hi"}); err == nil {
		t.Fatal("expected an error converting a non components v2 message")
	}
}
