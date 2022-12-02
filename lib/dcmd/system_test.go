package dcmd

import (
	"fmt"
	"testing"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/stretchr/testify/assert"
)

const (
	TestUserID    = 105487308693757952
	TestUserIDStr = "105487308693757952"
)

var (
	testSystem  *System
	testSession *discordgo.Session
)

type TestCommand struct{}

const (
	TestResponse = "Test Response"
)

func (e *TestCommand) ShortDescription() string { return "Test Description" }
func (e *TestCommand) Run(data *Data) (interface{}, error) {
	return TestResponse, nil
}

func SetupTestSystem() {
	testSystem = NewStandardSystem("!")
	testSystem.Root.AddCommand(&TestCommand{}, NewTrigger("test"))

	testSession = &discordgo.Session{
		State: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.SelfUser{
					User: &discordgo.User{
						ID: TestUserID,
					},
				},
			},
		},
	}

}

func TestFindPrefix(t *testing.T) {
	testChannelNoPriv := &discordgo.Channel{
		Type: discordgo.ChannelTypeGuildText,
	}

	testChannelPriv := &discordgo.Channel{
		Type: discordgo.ChannelTypeDM,
	}

	cases := []struct {
		channel             *discordgo.Channel
		msgContent          string
		expectedStripped    string
		shouldBeFound       bool
		expectedSource      TriggerSource
		expectedTriggerType TriggerType
		mentions            []*discordgo.User
	}{
		{testChannelNoPriv, "!cmd", "cmd", true, TriggerSourceGuild, TriggerTypePrefix, nil},
		{testChannelNoPriv, "cmd", "cmd", false, TriggerSourceGuild, TriggerTypePrefix, nil},
		{testChannelNoPriv, "<@" + TestUserIDStr + ">cmd", "cmd", true, TriggerSourceGuild, TriggerTypeMention, []*discordgo.User{{ID: TestUserID}}},
		{testChannelNoPriv, "<@" + TestUserIDStr + "> cmd", "cmd", true, TriggerSourceGuild, TriggerTypeMention, []*discordgo.User{{ID: TestUserID}}},
		{testChannelNoPriv, "<@" + TestUserIDStr + " cmd", "", false, TriggerSourceGuild, TriggerTypeMention, nil},
		{testChannelPriv, "cmd", "cmd", true, TriggerSourceDM, TriggerTypeDirect, nil},
	}

	for k, v := range cases {
		t.Run(fmt.Sprintf("#%d-p:%v-m:%v", k, v.channel == testChannelPriv, v.shouldBeFound), func(t *testing.T) {
			testData := &Data{
				Session: testSession,
				// Channel: v.channel,
				TraditionalTriggerData: &TraditionalTriggerData{
					Message: &discordgo.Message{
						Content:  v.msgContent,
						Mentions: v.mentions,
					},
				},
				Source: v.expectedSource,
			}

			if v.expectedSource != TriggerSourceDM {
				testData.TraditionalTriggerData.Message.GuildID = 1
			}

			found := testSystem.FindPrefix(testData)
			assert.Equal(t, v.shouldBeFound, found, "Should match test case")
			if !found {
				return
			}
			assert.Equal(t, v.expectedStripped, testData.TraditionalTriggerData.MessageStrippedPrefix, "Should be stripped off of prefix correctly")
			assert.Equal(t, v.expectedTriggerType, testData.TriggerType, "Should have the proper trigger type")
		})
	}
}
