package web

import (
	"github.com/jonas747/discordgo"
	"strings"
	"testing"
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
			t.Errorf("String case [%d] is valid, but shoulnd't be", i)
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

	g := &discordgo.Guild{
		Channels: []*discordgo.Channel{
			&discordgo.Channel{
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
