// GENERATED using events_gen.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package eventsystem

import (
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"runtime/debug"
)

type Event int

const (
	EventNewGuild                 Event = 0
	EventAll                      Event = 1
	EventAllPre                   Event = 2
	EventAllPost                  Event = 3
	EventMemberFetched            Event = 4
	EventChannelCreate            Event = 5
	EventChannelDelete            Event = 6
	EventChannelPinsUpdate        Event = 7
	EventChannelUpdate            Event = 8
	EventConnect                  Event = 9
	EventDisconnect               Event = 10
	EventGuildBanAdd              Event = 11
	EventGuildBanRemove           Event = 12
	EventGuildCreate              Event = 13
	EventGuildDelete              Event = 14
	EventGuildEmojisUpdate        Event = 15
	EventGuildIntegrationsUpdate  Event = 16
	EventGuildMemberAdd           Event = 17
	EventGuildMemberRemove        Event = 18
	EventGuildMemberUpdate        Event = 19
	EventGuildMembersChunk        Event = 20
	EventGuildRoleCreate          Event = 21
	EventGuildRoleDelete          Event = 22
	EventGuildRoleUpdate          Event = 23
	EventGuildUpdate              Event = 24
	EventMessageAck               Event = 25
	EventMessageCreate            Event = 26
	EventMessageDelete            Event = 27
	EventMessageDeleteBulk        Event = 28
	EventMessageReactionAdd       Event = 29
	EventMessageReactionRemove    Event = 30
	EventMessageReactionRemoveAll Event = 31
	EventMessageUpdate            Event = 32
	EventPresenceUpdate           Event = 33
	EventPresencesReplace         Event = 34
	EventRateLimit                Event = 35
	EventReady                    Event = 36
	EventRelationshipAdd          Event = 37
	EventRelationshipRemove       Event = 38
	EventResumed                  Event = 39
	EventTypingStart              Event = 40
	EventUserGuildSettingsUpdate  Event = 41
	EventUserNoteUpdate           Event = 42
	EventUserSettingsUpdate       Event = 43
	EventUserUpdate               Event = 44
	EventVoiceServerUpdate        Event = 45
	EventVoiceStateUpdate         Event = 46
)

var EventNames = []string{
	"NewGuild",
	"All",
	"AllPre",
	"AllPost",
	"MemberFetched",
	"ChannelCreate",
	"ChannelDelete",
	"ChannelPinsUpdate",
	"ChannelUpdate",
	"Connect",
	"Disconnect",
	"GuildBanAdd",
	"GuildBanRemove",
	"GuildCreate",
	"GuildDelete",
	"GuildEmojisUpdate",
	"GuildIntegrationsUpdate",
	"GuildMemberAdd",
	"GuildMemberRemove",
	"GuildMemberUpdate",
	"GuildMembersChunk",
	"GuildRoleCreate",
	"GuildRoleDelete",
	"GuildRoleUpdate",
	"GuildUpdate",
	"MessageAck",
	"MessageCreate",
	"MessageDelete",
	"MessageDeleteBulk",
	"MessageReactionAdd",
	"MessageReactionRemove",
	"MessageReactionRemoveAll",
	"MessageUpdate",
	"PresenceUpdate",
	"PresencesReplace",
	"RateLimit",
	"Ready",
	"RelationshipAdd",
	"RelationshipRemove",
	"Resumed",
	"TypingStart",
	"UserGuildSettingsUpdate",
	"UserNoteUpdate",
	"UserSettingsUpdate",
	"UserUpdate",
	"VoiceServerUpdate",
	"VoiceStateUpdate",
}

func (e Event) String() string {
	return EventNames[e]
}

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
	EventMessageReactionRemoveAll,
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
	EventUserNoteUpdate,
	EventUserSettingsUpdate,
	EventUserUpdate,
	EventVoiceServerUpdate,
	EventVoiceStateUpdate,
}

var handlers = make([][]*Handler, 47)

type EventDataContainer struct {
	ChannelCreate            *discordgo.ChannelCreate
	ChannelDelete            *discordgo.ChannelDelete
	ChannelPinsUpdate        *discordgo.ChannelPinsUpdate
	ChannelUpdate            *discordgo.ChannelUpdate
	Connect                  *discordgo.Connect
	Disconnect               *discordgo.Disconnect
	GuildBanAdd              *discordgo.GuildBanAdd
	GuildBanRemove           *discordgo.GuildBanRemove
	GuildCreate              *discordgo.GuildCreate
	GuildDelete              *discordgo.GuildDelete
	GuildEmojisUpdate        *discordgo.GuildEmojisUpdate
	GuildIntegrationsUpdate  *discordgo.GuildIntegrationsUpdate
	GuildMemberAdd           *discordgo.GuildMemberAdd
	GuildMemberRemove        *discordgo.GuildMemberRemove
	GuildMemberUpdate        *discordgo.GuildMemberUpdate
	GuildMembersChunk        *discordgo.GuildMembersChunk
	GuildRoleCreate          *discordgo.GuildRoleCreate
	GuildRoleDelete          *discordgo.GuildRoleDelete
	GuildRoleUpdate          *discordgo.GuildRoleUpdate
	GuildUpdate              *discordgo.GuildUpdate
	MessageAck               *discordgo.MessageAck
	MessageCreate            *discordgo.MessageCreate
	MessageDelete            *discordgo.MessageDelete
	MessageDeleteBulk        *discordgo.MessageDeleteBulk
	MessageReactionAdd       *discordgo.MessageReactionAdd
	MessageReactionRemove    *discordgo.MessageReactionRemove
	MessageReactionRemoveAll *discordgo.MessageReactionRemoveAll
	MessageUpdate            *discordgo.MessageUpdate
	PresenceUpdate           *discordgo.PresenceUpdate
	PresencesReplace         *discordgo.PresencesReplace
	RateLimit                *discordgo.RateLimit
	Ready                    *discordgo.Ready
	RelationshipAdd          *discordgo.RelationshipAdd
	RelationshipRemove       *discordgo.RelationshipRemove
	Resumed                  *discordgo.Resumed
	TypingStart              *discordgo.TypingStart
	UserGuildSettingsUpdate  *discordgo.UserGuildSettingsUpdate
	UserNoteUpdate           *discordgo.UserNoteUpdate
	UserSettingsUpdate       *discordgo.UserSettingsUpdate
	UserUpdate               *discordgo.UserUpdate
	VoiceServerUpdate        *discordgo.VoiceServerUpdate
	VoiceStateUpdate         *discordgo.VoiceStateUpdate
}

func HandleEvent(s *discordgo.Session, evt interface{}) {

	var evtData = &EventData{
		EventDataContainer: &EventDataContainer{},
		EvtInterface:       evt,
	}

	switch t := evt.(type) {
	case *discordgo.ChannelCreate:
		evtData.ChannelCreate = t
		evtData.Type = Event(5)
	case *discordgo.ChannelDelete:
		evtData.ChannelDelete = t
		evtData.Type = Event(6)
	case *discordgo.ChannelPinsUpdate:
		evtData.ChannelPinsUpdate = t
		evtData.Type = Event(7)
	case *discordgo.ChannelUpdate:
		evtData.ChannelUpdate = t
		evtData.Type = Event(8)
	case *discordgo.Connect:
		evtData.Connect = t
		evtData.Type = Event(9)
	case *discordgo.Disconnect:
		evtData.Disconnect = t
		evtData.Type = Event(10)
	case *discordgo.GuildBanAdd:
		evtData.GuildBanAdd = t
		evtData.Type = Event(11)
	case *discordgo.GuildBanRemove:
		evtData.GuildBanRemove = t
		evtData.Type = Event(12)
	case *discordgo.GuildCreate:
		evtData.GuildCreate = t
		evtData.Type = Event(13)
	case *discordgo.GuildDelete:
		evtData.GuildDelete = t
		evtData.Type = Event(14)
	case *discordgo.GuildEmojisUpdate:
		evtData.GuildEmojisUpdate = t
		evtData.Type = Event(15)
	case *discordgo.GuildIntegrationsUpdate:
		evtData.GuildIntegrationsUpdate = t
		evtData.Type = Event(16)
	case *discordgo.GuildMemberAdd:
		evtData.GuildMemberAdd = t
		evtData.Type = Event(17)
	case *discordgo.GuildMemberRemove:
		evtData.GuildMemberRemove = t
		evtData.Type = Event(18)
	case *discordgo.GuildMemberUpdate:
		evtData.GuildMemberUpdate = t
		evtData.Type = Event(19)
	case *discordgo.GuildMembersChunk:
		evtData.GuildMembersChunk = t
		evtData.Type = Event(20)
	case *discordgo.GuildRoleCreate:
		evtData.GuildRoleCreate = t
		evtData.Type = Event(21)
	case *discordgo.GuildRoleDelete:
		evtData.GuildRoleDelete = t
		evtData.Type = Event(22)
	case *discordgo.GuildRoleUpdate:
		evtData.GuildRoleUpdate = t
		evtData.Type = Event(23)
	case *discordgo.GuildUpdate:
		evtData.GuildUpdate = t
		evtData.Type = Event(24)
	case *discordgo.MessageAck:
		evtData.MessageAck = t
		evtData.Type = Event(25)
	case *discordgo.MessageCreate:
		evtData.MessageCreate = t
		evtData.Type = Event(26)
	case *discordgo.MessageDelete:
		evtData.MessageDelete = t
		evtData.Type = Event(27)
	case *discordgo.MessageDeleteBulk:
		evtData.MessageDeleteBulk = t
		evtData.Type = Event(28)
	case *discordgo.MessageReactionAdd:
		evtData.MessageReactionAdd = t
		evtData.Type = Event(29)
	case *discordgo.MessageReactionRemove:
		evtData.MessageReactionRemove = t
		evtData.Type = Event(30)
	case *discordgo.MessageReactionRemoveAll:
		evtData.MessageReactionRemoveAll = t
		evtData.Type = Event(31)
	case *discordgo.MessageUpdate:
		evtData.MessageUpdate = t
		evtData.Type = Event(32)
	case *discordgo.PresenceUpdate:
		evtData.PresenceUpdate = t
		evtData.Type = Event(33)
	case *discordgo.PresencesReplace:
		evtData.PresencesReplace = t
		evtData.Type = Event(34)
	case *discordgo.RateLimit:
		evtData.RateLimit = t
		evtData.Type = Event(35)
	case *discordgo.Ready:
		evtData.Ready = t
		evtData.Type = Event(36)
	case *discordgo.RelationshipAdd:
		evtData.RelationshipAdd = t
		evtData.Type = Event(37)
	case *discordgo.RelationshipRemove:
		evtData.RelationshipRemove = t
		evtData.Type = Event(38)
	case *discordgo.Resumed:
		evtData.Resumed = t
		evtData.Type = Event(39)
	case *discordgo.TypingStart:
		evtData.TypingStart = t
		evtData.Type = Event(40)
	case *discordgo.UserGuildSettingsUpdate:
		evtData.UserGuildSettingsUpdate = t
		evtData.Type = Event(41)
	case *discordgo.UserNoteUpdate:
		evtData.UserNoteUpdate = t
		evtData.Type = Event(42)
	case *discordgo.UserSettingsUpdate:
		evtData.UserSettingsUpdate = t
		evtData.Type = Event(43)
	case *discordgo.UserUpdate:
		evtData.UserUpdate = t
		evtData.Type = Event(44)
	case *discordgo.VoiceServerUpdate:
		evtData.VoiceServerUpdate = t
		evtData.Type = Event(45)
	case *discordgo.VoiceStateUpdate:
		evtData.VoiceStateUpdate = t
		evtData.Type = Event(46)
	default:
		return
	}

	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			logrus.WithField(logrus.ErrorKey, err).WithField("evt", evtData.Type.String()).Error("Recovered from panic in event handler\n" + stack)
		}
	}()

	ctx := context.WithValue(context.Background(), common.ContextKeyDiscordSession, s)
	evtData.ctx = ctx
	EmitEvent(evtData, EventAllPre)
	EmitEvent(evtData, evtData.Type)
	EmitEvent(evtData, EventAllPost)
}
