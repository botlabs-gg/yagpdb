package web

import (
	"strings"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
)

type StringTestStruct struct {
	Length    string `valid:",1,12"`
	TrimSpace string `valid:",1,12,trimspace"`
}

var (
	stringTestCases = []struct {
		Struct *StringTestStruct
		Valid  bool
	}{
		{ // 0
			Struct: &StringTestStruct{
				Length:    "123",
				TrimSpace: "aa  ",
			},
			Valid: true,
		}, { // 1
			Struct: &StringTestStruct{
				Length:    "",
				TrimSpace: "aa",
			},
			Valid: false,
		}, { // 2
			Struct: &StringTestStruct{
				Length:    "aaaaaaaaaaaaaaaaa",
				TrimSpace: "aa",
			},
			Valid: false,
		}, { // 3
			Struct: &StringTestStruct{
				Length:    "aa",
				TrimSpace: " ",
			},
			Valid: false,
		},
	}
)

func TestValidationString(t *testing.T) {
	for i, v := range stringTestCases {
		ok := ValidateForm(nil, TemplateData(make(map[string]interface{})), v.Struct)
		if ok && !v.Valid {
			t.Errorf("String case [%d] is valid, but shouldn't be", i)
		} else if !ok && v.Valid {
			t.Errorf("String case [%d] is not valid, but should be", i)
		} else if ok && strings.TrimSpace(v.Struct.TrimSpace) != v.Struct.TrimSpace {
			t.Errorf("String case [%d] is not trimmed properly", i)
		}
	}
}

type ChannelTestStruct struct {
	ChannelNotEmpty   string `valid:"channel,false"`
	ChannelAllowEmpty string `valid:"channel,true"`
}

var (
	channelTestCases = []struct {
		Struct *ChannelTestStruct
		Valid  bool
	}{
		{
			Struct: &ChannelTestStruct{
				ChannelNotEmpty:   "1",
				ChannelAllowEmpty: "",
			},
			Valid: true,
		},
		{
			Struct: &ChannelTestStruct{
				ChannelNotEmpty:   "1",
				ChannelAllowEmpty: "0",
			},
			Valid: true,
		},
		{
			Struct: &ChannelTestStruct{
				ChannelNotEmpty:   "",
				ChannelAllowEmpty: "",
			},
			Valid: false,
		},
		{
			Struct: &ChannelTestStruct{
				ChannelNotEmpty:   "5",
				ChannelAllowEmpty: "",
			},
			Valid: false,
		},
		{
			Struct: &ChannelTestStruct{
				ChannelNotEmpty:   "0",
				ChannelAllowEmpty: "5",
			},
			Valid: false,
		},
	}
)

func TestValidationChannel(t *testing.T) {

	g := &dstate.GuildSet{
		Channels: []dstate.ChannelState{
			{
				ID: 1,
			},
		},
	}

	for i, v := range channelTestCases {
		ok := ValidateForm(g, TemplateData(make(map[string]interface{})), v.Struct)
		if ok && !v.Valid {
			t.Errorf("Channel case [%d] is valid, but shoulnd't be", i)
		} else if !ok && v.Valid {
			t.Errorf("Channel case [%d] is not valid, but should be", i)
		}
	}
}

func TestValidationNestedSlice(t *testing.T) {
	type NestedString struct {
		Msg string `valid:",1,8,trimspace"`
	}

	type NestedSlice struct {
		Msgs []NestedString `valid:"traverse"`
	}

	testCases := []struct {
		Struct *NestedSlice
		Valid  bool
	}{
		{ // 0
			Struct: &NestedSlice{
				[]NestedString{
					{"aa"},
					{"aa   "},
				},
			},
			Valid: true,
		}, { // 1
			Struct: &NestedSlice{
				[]NestedString{
					{"aa"},
					{"123456789"},
				},
			},
			Valid: false,
		},
	}

	allTrimmed := func(n *NestedSlice) bool {
		for _, s := range n.Msgs {
			if strings.TrimSpace(s.Msg) != s.Msg {
				return false
			}
		}
		return true
	}

	for i, v := range testCases {
		ok := ValidateForm(nil, TemplateData(make(map[string]interface{})), v.Struct)
		if ok && !v.Valid {
			t.Errorf("String case [%d] is valid, but shouldn't be", i)
		} else if !ok && v.Valid {
			t.Errorf("String case [%d] is not valid, but should be", i)
		} else if !allTrimmed(v.Struct) {
			t.Errorf("String case [%d] is not trimmed properly", i)
		}
	}
}
