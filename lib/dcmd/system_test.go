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

func TestFindPrefixWithPrefixTriggerDisabled(t *testing.T) {
	guildChannel := &discordgo.Channel{Type: discordgo.ChannelTypeGuildText}
	dmChannel := &discordgo.Channel{Type: discordgo.ChannelTypeDM}

	cases := []struct {
		name                string
		channel             *discordgo.Channel
		msgContent          string
		mentions            []*discordgo.User
		source              TriggerSource
		shouldBeFound       bool
		expectedTriggerType TriggerType
	}{
		{"prefix is ignored", guildChannel, "!test", nil, TriggerSourceGuild, false, TriggerTypePrefix},
		{"mention still works", guildChannel, "<@" + TestUserIDStr + "> test", []*discordgo.User{{ID: TestUserID}}, TriggerSourceGuild, true, TriggerTypeMention},
		{"dm still works", dmChannel, "test", nil, TriggerSourceDM, true, TriggerTypeDirect},
	}

	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			sys := NewStandardSystem("!")
			sys.DisablePrefixTrigger = true

			newData := func() *Data {
				d := &Data{
					Session: testSession,
					TraditionalTriggerData: &TraditionalTriggerData{
						Message: &discordgo.Message{Content: v.msgContent, Mentions: v.mentions},
					},
					Source: v.source,
				}
				if v.source != TriggerSourceDM {
					d.TraditionalTriggerData.Message.GuildID = 1
				}
				return d
			}

			data := newData()
			assert.Equal(t, v.shouldBeFound, sys.FindPrefix(data), "FindPrefix")
			if v.shouldBeFound {
				assert.Equal(t, v.expectedTriggerType, data.TriggerType, "FindPrefix trigger type")
			}

			data = newData()
			assert.Equal(t, v.shouldBeFound, sys.FindPrefixWithPrefetched(data, "!"), "FindPrefixWithPrefetched")
			if v.shouldBeFound {
				assert.Equal(t, v.expectedTriggerType, data.TriggerType, "FindPrefixWithPrefetched trigger type")
			}
		})
	}
}

// An empty prefix must never match every message.
func TestFindPrefixWithPrefetchedEmptyPrefix(t *testing.T) {
	data := &Data{
		Session: testSession,
		TraditionalTriggerData: &TraditionalTriggerData{
			Message: &discordgo.Message{Content: "not a command", GuildID: 1},
		},
		Source: TriggerSourceGuild,
	}

	assert.False(t, testSystem.FindPrefixWithPrefetched(data, ""))
}
