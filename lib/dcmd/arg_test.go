package dcmd

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/stretchr/testify/assert"
)

func TestIntArg(t *testing.T) {
	part := "123"
	expected := int64(123)

	assert.True(t, Int.Matches(nil, part), "Should match")

	v, err := Int.ParseFromMessage(nil, part, nil)
	assert.NoError(t, err, "Should parse sucessfully")
	assert.Equal(t, v, expected, "Should be equal")

	assert.False(t, Int.Matches(nil, "12hello21"), "Should not match")
}

func TestFloatArg(t *testing.T) {
	part := "12.3"
	expected := float64(12.3)

	assert.True(t, Float.Matches(nil, part), "Should match")

	v, err := Float.ParseFromMessage(nil, part, nil)
	assert.NoError(t, err, "Should parse sucessfully")
	assert.Equal(t, v, expected, "Should be equal")

	assert.False(t, Float.Matches(nil, "1.2hello21"), "Should not match")
}

func TestUserIDArg(t *testing.T) {
	d := &Data{
		TraditionalTriggerData: &TraditionalTriggerData{
			Message: &discordgo.Message{
				Mentions: []*discordgo.User{},
			},
		},
	}

	cases := []struct {
		part   string
		match  bool
		result int64
	}{
		{"123", true, 123},
		{"hello", false, 321},
		{"<@105487308693757952>", true, 105487308693757952},
	}

	for _, c := range cases {
		t.Run("case_"+c.part, func(t *testing.T) {
			arg := &UserIDArg{}
			matches := arg.Matches(nil, c.part)
			assert.Equal(t, c.match, matches, "Incorrect match")
			if matches {
				parsed, err := arg.ParseFromMessage(nil, c.part, d)
				assert.NoError(t, err, "Should parse sucessfully")
				assert.Equal(t, c.result, parsed)
			}
		})
	}
}
