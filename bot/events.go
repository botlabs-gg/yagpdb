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
	
	EventNewGuild Event = 0
	EventAll Event = 1
	EventAllPre Event = 2
	EventAllPost Event = 3
	EventMemberFetched Event = 4
	EventChannelCreate Event = 5
	EventChannelDelete Event = 6
	EventChannelPinsUpdate Event = 7
	EventChannelUpdate Event = 8
	EventConnect Event = 9
	EventDisconnect Event = 10
	EventGuildBanAdd Event = 11
	EventGuildBanRemove Event = 12
	EventGuildCreate Event = 13
	EventGuildDelete Event = 14
	EventGuildEmojisUpdate Event = 15
	EventGuildIntegrationsUpdate Event = 16
	EventGuildMemberAdd Event = 17
	EventGuildMemberRemove Event = 18
	EventGuildMemberUpdate Event = 19
	EventGuildMembersChunk Event = 20
	EventGuildRoleCreate Event = 21
	EventGuildRoleDelete Event = 22
	EventGuildRoleUpdate Event = 23
	EventGuildUpdate Event = 24
	EventMessageAck Event = 25
	EventMessageCreate Event = 26
	EventMessageDelete Event = 27
	EventMessageDeleteBulk Event = 28
	EventMessageReactionAdd Event = 29
	EventMessageReactionRemove Event = 30
	EventMessageUpdate Event = 31
	EventPresenceUpdate Event = 32
	EventPresencesReplace Event = 33
	EventRateLimit Event = 34
	EventReady Event = 35
	EventRelationshipAdd Event = 36
	EventRelationshipRemove Event = 37
	EventResumed Event = 38
	EventTypingStart Event = 39
	EventUserGuildSettingsUpdate Event = 40
	EventUserSettingsUpdate Event = 41
	EventUserUpdate Event = 42
	EventVoiceServerUpdate Event = 43
	EventVoiceStateUpdate Event = 44
)

var AllDiscordEvents = []Event{ 
	EventChannelCreate,
	EventChannelDelete,
	EventChannelPinsUpdate,
	EventChannelUpdate,
	EventConnect,
	EventDisconnect,
	EventGuildBanAdd,
	EventGuildBanRemove,
	EventGuildCreate,
	EventGuildDelete,
	EventGuildEmojisUpdate,
	EventGuildIntegrationsUpdate,
	EventGuildMemberAdd,
	EventGuildMemberRemove,
	EventGuildMemberUpdate,
	EventGuildMembersChunk,
	EventGuildRoleCreate,
	EventGuildRoleDelete,
	EventGuildRoleUpdate,
	EventGuildUpdate,
	EventMessageAck,
	EventMessageCreate,
	EventMessageDelete,
	EventMessageDeleteBulk,
	EventMessageReactionAdd,
	EventMessageReactionRemove,
	EventMessageUpdate,
	EventPresenceUpdate,
	EventPresencesReplace,
	EventRateLimit,
	EventReady,
	EventRelationshipAdd,
	EventRelationshipRemove,
	EventResumed,
	EventTypingStart,
	EventUserGuildSettingsUpdate,
	EventUserSettingsUpdate,
	EventUserUpdate,
	EventVoiceServerUpdate,
	EventVoiceStateUpdate,
}

type Handler func(ctx context.Context, evt interface{})
var handlers = make([][]*Handler, 45)

func handleEvent(s *discordgo.Session, evt interface{}){

	evtId := -10
	name := ""

	switch evt.(type){ 
	case *discordgo.ChannelCreate:
		evtId = 5
		name = "ChannelCreate"
	case *discordgo.ChannelDelete:
		evtId = 6
		name = "ChannelDelete"
	case *discordgo.ChannelPinsUpdate:
		evtId = 7
		name = "ChannelPinsUpdate"
	case *discordgo.ChannelUpdate:
		evtId = 8
		name = "ChannelUpdate"
	case *discordgo.Connect:
		evtId = 9
		name = "Connect"
	case *discordgo.Disconnect:
		evtId = 10
		name = "Disconnect"
	case *discordgo.GuildBanAdd:
		evtId = 11
		name = "GuildBanAdd"
	case *discordgo.GuildBanRemove:
		evtId = 12
		name = "GuildBanRemove"
	case *discordgo.GuildCreate:
		evtId = 13
		name = "GuildCreate"
	case *discordgo.GuildDelete:
		evtId = 14
		name = "GuildDelete"
	case *discordgo.GuildEmojisUpdate:
		evtId = 15
		name = "GuildEmojisUpdate"
	case *discordgo.GuildIntegrationsUpdate:
		evtId = 16
		name = "GuildIntegrationsUpdate"
	case *discordgo.GuildMemberAdd:
		evtId = 17
		name = "GuildMemberAdd"
	case *discordgo.GuildMemberRemove:
		evtId = 18
		name = "GuildMemberRemove"
	case *discordgo.GuildMemberUpdate:
		evtId = 19
		name = "GuildMemberUpdate"
	case *discordgo.GuildMembersChunk:
		evtId = 20
		name = "GuildMembersChunk"
	case *discordgo.GuildRoleCreate:
		evtId = 21
		name = "GuildRoleCreate"
	case *discordgo.GuildRoleDelete:
		evtId = 22
		name = "GuildRoleDelete"
	case *discordgo.GuildRoleUpdate:
		evtId = 23
		name = "GuildRoleUpdate"
	case *discordgo.GuildUpdate:
		evtId = 24
		name = "GuildUpdate"
	case *discordgo.MessageAck:
		evtId = 25
		name = "MessageAck"
	case *discordgo.MessageCreate:
		evtId = 26
		name = "MessageCreate"
	case *discordgo.MessageDelete:
		evtId = 27
		name = "MessageDelete"
	case *discordgo.MessageDeleteBulk:
		evtId = 28
		name = "MessageDeleteBulk"
	case *discordgo.MessageReactionAdd:
		evtId = 29
		name = "MessageReactionAdd"
	case *discordgo.MessageReactionRemove:
		evtId = 30
		name = "MessageReactionRemove"
	case *discordgo.MessageUpdate:
		evtId = 31
		name = "MessageUpdate"
	case *discordgo.PresenceUpdate:
		evtId = 32
		name = "PresenceUpdate"
	case *discordgo.PresencesReplace:
		evtId = 33
		name = "PresencesReplace"
	case *discordgo.RateLimit:
		evtId = 34
		name = "RateLimit"
	case *discordgo.Ready:
		evtId = 35
		name = "Ready"
	case *discordgo.RelationshipAdd:
		evtId = 36
		name = "RelationshipAdd"
	case *discordgo.RelationshipRemove:
		evtId = 37
		name = "RelationshipRemove"
	case *discordgo.Resumed:
		evtId = 38
		name = "Resumed"
	case *discordgo.TypingStart:
		evtId = 39
		name = "TypingStart"
	case *discordgo.UserGuildSettingsUpdate:
		evtId = 40
		name = "UserGuildSettingsUpdate"
	case *discordgo.UserSettingsUpdate:
		evtId = 41
		name = "UserSettingsUpdate"
	case *discordgo.UserUpdate:
		evtId = 42
		name = "UserUpdate"
	case *discordgo.VoiceServerUpdate:
		evtId = 43
		name = "VoiceServerUpdate"
	case *discordgo.VoiceStateUpdate:
		evtId = 44
		name = "VoiceStateUpdate"
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
