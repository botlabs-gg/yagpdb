// GENERATED using yagpdb/cmd/gen/bot_wrappers.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package bot

import (
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"runtime/debug"
)

type Event int

const (
	
	EventChannelCreate Event = 0
	EventChannelUpdate Event = 1
	EventChannelDelete Event = 2
	EventChannelPinsUpdate Event = 3
	EventGuildCreate Event = 4
	EventGuildUpdate Event = 5
	EventGuildDelete Event = 6
	EventGuildBanAdd Event = 7
	EventGuildBanRemove Event = 8
	EventGuildMemberAdd Event = 9
	EventGuildMemberUpdate Event = 10
	EventGuildMemberRemove Event = 11
	EventGuildMembersChunk Event = 12
	EventGuildRoleCreate Event = 13
	EventGuildRoleUpdate Event = 14
	EventGuildRoleDelete Event = 15
	EventGuildIntegrationsUpdate Event = 16
	EventGuildEmojisUpdate Event = 17
	EventMessageAck Event = 18
	EventMessageCreate Event = 19
	EventMessageUpdate Event = 20
	EventMessageDelete Event = 21
	EventMessageDeleteBulk Event = 22
	EventPresenceUpdate Event = 23
	EventPresencesReplace Event = 24
	EventReady Event = 25
	EventUserUpdate Event = 26
	EventUserSettingsUpdate Event = 27
	EventUserGuildSettingsUpdate Event = 28
	EventTypingStart Event = 29
	EventVoiceServerUpdate Event = 30
	EventVoiceStateUpdate Event = 31
	EventResumed Event = 32
	EventNewGuild Event = 33
	EventAll Event = 34
	EventAllPre Event = 35
	EventAllPost Event = 36
)

var AllDiscordEvents = []Event{ 
	EventChannelCreate,
	EventChannelUpdate,
	EventChannelDelete,
	EventChannelPinsUpdate,
	EventGuildCreate,
	EventGuildUpdate,
	EventGuildDelete,
	EventGuildBanAdd,
	EventGuildBanRemove,
	EventGuildMemberAdd,
	EventGuildMemberUpdate,
	EventGuildMemberRemove,
	EventGuildMembersChunk,
	EventGuildRoleCreate,
	EventGuildRoleUpdate,
	EventGuildRoleDelete,
	EventGuildIntegrationsUpdate,
	EventGuildEmojisUpdate,
	EventMessageAck,
	EventMessageCreate,
	EventMessageUpdate,
	EventMessageDelete,
	EventMessageDeleteBulk,
	EventPresenceUpdate,
	EventPresencesReplace,
	EventReady,
	EventUserUpdate,
	EventUserSettingsUpdate,
	EventUserGuildSettingsUpdate,
	EventTypingStart,
	EventVoiceServerUpdate,
	EventVoiceStateUpdate,
	EventResumed,
}

type Handler func(ctx context.Context, evt interface{})
var handlers = make([][]*Handler, 37)

func handleEvent(s *discordgo.Session, evt interface{}){

	evtId := -10
	name := ""

	switch evt.(type){ 
	case *discordgo.ChannelCreate:
		evtId = 0
		name = "ChannelCreate"
	case *discordgo.ChannelUpdate:
		evtId = 1
		name = "ChannelUpdate"
	case *discordgo.ChannelDelete:
		evtId = 2
		name = "ChannelDelete"
	case *discordgo.ChannelPinsUpdate:
		evtId = 3
		name = "ChannelPinsUpdate"
	case *discordgo.GuildCreate:
		evtId = 4
		name = "GuildCreate"
	case *discordgo.GuildUpdate:
		evtId = 5
		name = "GuildUpdate"
	case *discordgo.GuildDelete:
		evtId = 6
		name = "GuildDelete"
	case *discordgo.GuildBanAdd:
		evtId = 7
		name = "GuildBanAdd"
	case *discordgo.GuildBanRemove:
		evtId = 8
		name = "GuildBanRemove"
	case *discordgo.GuildMemberAdd:
		evtId = 9
		name = "GuildMemberAdd"
	case *discordgo.GuildMemberUpdate:
		evtId = 10
		name = "GuildMemberUpdate"
	case *discordgo.GuildMemberRemove:
		evtId = 11
		name = "GuildMemberRemove"
	case *discordgo.GuildMembersChunk:
		evtId = 12
		name = "GuildMembersChunk"
	case *discordgo.GuildRoleCreate:
		evtId = 13
		name = "GuildRoleCreate"
	case *discordgo.GuildRoleUpdate:
		evtId = 14
		name = "GuildRoleUpdate"
	case *discordgo.GuildRoleDelete:
		evtId = 15
		name = "GuildRoleDelete"
	case *discordgo.GuildIntegrationsUpdate:
		evtId = 16
		name = "GuildIntegrationsUpdate"
	case *discordgo.GuildEmojisUpdate:
		evtId = 17
		name = "GuildEmojisUpdate"
	case *discordgo.MessageAck:
		evtId = 18
		name = "MessageAck"
	case *discordgo.MessageCreate:
		evtId = 19
		name = "MessageCreate"
	case *discordgo.MessageUpdate:
		evtId = 20
		name = "MessageUpdate"
	case *discordgo.MessageDelete:
		evtId = 21
		name = "MessageDelete"
	case *discordgo.MessageDeleteBulk:
		evtId = 22
		name = "MessageDeleteBulk"
	case *discordgo.PresenceUpdate:
		evtId = 23
		name = "PresenceUpdate"
	case *discordgo.PresencesReplace:
		evtId = 24
		name = "PresencesReplace"
	case *discordgo.Ready:
		evtId = 25
		name = "Ready"
	case *discordgo.UserUpdate:
		evtId = 26
		name = "UserUpdate"
	case *discordgo.UserSettingsUpdate:
		evtId = 27
		name = "UserSettingsUpdate"
	case *discordgo.UserGuildSettingsUpdate:
		evtId = 28
		name = "UserGuildSettingsUpdate"
	case *discordgo.TypingStart:
		evtId = 29
		name = "TypingStart"
	case *discordgo.VoiceServerUpdate:
		evtId = 30
		name = "VoiceServerUpdate"
	case *discordgo.VoiceStateUpdate:
		evtId = 31
		name = "VoiceStateUpdate"
	case *discordgo.Resumed:
		evtId = 32
		name = "Resumed"
	default:
		return
	}

	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			logrus.WithField(logrus.ErrorKey, err).WithField("evt", name).Error("Recovered from panic in event handler\n" + stack)
		}
	}()

	ctx := context.WithValue(context.Background(), ContextKeySession, s)

	EmitEvent(ctx, EventAllPre, evt)
	EmitEvent(ctx, Event(evtId), evt)
	EmitEvent(ctx, EventAllPost, evt)
}
