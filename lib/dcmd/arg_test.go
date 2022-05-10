package dcmd

import (
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/stretchr/testify/assert"
)

func TestIntArg(t *testing.T) {
	part := "123"
	expected := int64(123)

	assert.Equal(t, CompatibilityGood, Int.CheckCompatibility(nil, part), "Should have excellent compatibility")

	v, err := Int.ParseFromMessage(nil, part, nil)
	assert.NoError(t, err, "Should parse sucessfully")
	assert.Equal(t, v, expected, "Should be equal")

	assert.Equal(t, Incompatible, Int.CheckCompatibility(nil, "12hello21"), "Should be incompatible")
}

func TestFloatArg(t *testing.T) {
	part := "12.3"
	expected := float64(12.3)

	assert.Equal(t, CompatibilityGood, Float.CheckCompatibility(nil, part), "Should have excellent compatibility")

	v, err := Float.ParseFromMessage(nil, part, nil)
	assert.NoError(t, err, "Should parse sucessfully")
	assert.Equal(t, v, expected, "Should be equal")

	assert.Equal(t, Incompatible, Float.CheckCompatibility(nil, "1.2hello21"), "Should be incompatible")
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
		compat CompatibilityResult
		result int64
	}{
		{"123", CompatibilityGood, 123},
		{"hello", Incompatible, 321},
		{"<@105487308693757952>", CompatibilityGood, 105487308693757952},
	}

	for _, c := range cases {
		t.Run("case_"+c.part, func(t *testing.T) {
			arg := &UserIDArg{}
			compat := arg.CheckCompatibility(nil, c.part)
			assert.Equal(t, c.compat, compat, "Compitability result doesn't match")
			if compat != Incompatible {
				parsed, err := arg.ParseFromMessage(nil, c.part, d)
				assert.NoError(t, err, "Should parse sucessfully")
				assert.Equal(t, c.result, parsed)
			}
		})
	}
}
