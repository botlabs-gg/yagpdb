package cah

import (
	"slices"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/cardsagainstdiscord"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func pickerMenus(components []discordgo.TopLevelComponent) []discordgo.SelectMenu {
	var menus []discordgo.SelectMenu
	for _, c := range components[0].(discordgo.Container).Components {
		if row, ok := c.(discordgo.ActionsRow); ok {
			for _, v := range row.Components {
				if menu, ok := v.(discordgo.SelectMenu); ok {
					menus = append(menus, menu)
				}
			}
		}
	}

	return menus
}

func TestNewPickerPicksDefaults(t *testing.T) {
	conf := DefaultConfig(1)
	conf.DefaultPacks = []string{"main", "ours"}
	conf.DisabledPacks = []string{"redbox"}
	custom := []*cardsagainstdiscord.CardPack{{Name: "ours", Description: "our own"}}

	picker := &gamePicker{}
	menus := pickerMenus(newPicker(conf, custom, picker))

	if !slices.Equal(picker.Builtin, []string{"main"}) || !slices.Equal(picker.Custom, []string{"ours"}) {
		t.Fatalf("expected the defaults to be picked, got %v and %v", picker.Builtin, picker.Custom)
	}

	if len(menus) != 2 {
		t.Fatalf("expected a built-in and a custom pack menu, got %d", len(menus))
	}

	for _, menu := range menus {
		if len(menu.CustomID) > 100 || *menu.MinValues != 0 || menu.MaxValues != len(menu.Options) {
			t.Fatalf("unexpected menu limits: %s %d %d", menu.CustomID, *menu.MinValues, menu.MaxValues)
		}

		for _, option := range menu.Options {
			if option.Value == "redbox" {
				t.Fatal("disabled packs should not be offered")
			}
			if option.Default != (option.Value == "main" || option.Value == "ours") {
				t.Fatalf("unexpected default for %s", option.Value)
			}
		}
	}
}

func TestPickerWithoutCustomPacks(t *testing.T) {
	menus := pickerMenus(newPicker(DefaultConfig(1), nil, &gamePicker{}))
	if len(menus) != 1 || len(menus[0].Options) != len(cardsagainstdiscord.Packs) {
		t.Fatalf("expected one menu with every built-in pack, got %d menus", len(menus))
	}
}

func TestPickerShowsDefaultsFirst(t *testing.T) {
	conf := DefaultConfig(1)
	conf.DefaultPacks = []string{"sanity", "bluebox"}

	options := pickerMenus(newPicker(conf, nil, &gamePicker{}))[0].Options
	if options[0].Value != "sanity" || options[1].Value != "bluebox" || options[2].Value != BuiltinPackNames()[0] {
		t.Fatalf("expected the defaults first and the rest in order, got %s, %s, %s", options[0].Value, options[1].Value, options[2].Value)
	}

	options = pickerMenus(newPicker(DefaultConfig(1), nil, &gamePicker{}))[0].Options
	if options[0].Value != "main" {
		t.Fatalf("expected main first by default, got %s", options[0].Value)
	}
}
