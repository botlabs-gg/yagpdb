// GENERATED using events_gen.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package eventsystem

import (
	"github.com/jonas747/discordgo"
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
	EventWebhooksUpdate           Event = 47
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
	"WebhooksUpdate",
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
	EventWebhooksUpdate,
}

var AllEvents = []Event{
	EventNewGuild,
	EventAll,
	EventAllPre,
	EventAllPost,
	EventMemberFetched,
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
	EventWebhooksUpdate,
}

var handlers = make([][][]*Handler, 48)

func (data *EventData) ChannelCreate() *discordgo.ChannelCreate {
	return data.EvtInterface.(*discordgo.ChannelCreate)
}
func (data *EventData) ChannelDelete() *discordgo.ChannelDelete {
	return data.EvtInterface.(*discordgo.ChannelDelete)
}
func (data *EventData) ChannelPinsUpdate() *discordgo.ChannelPinsUpdate {
	return data.EvtInterface.(*discordgo.ChannelPinsUpdate)
}
func (data *EventData) ChannelUpdate() *discordgo.ChannelUpdate {
	return data.EvtInterface.(*discordgo.ChannelUpdate)
}
func (data *EventData) Connect() *discordgo.Connect {
	return data.EvtInterface.(*discordgo.Connect)
}
func (data *EventData) Disconnect() *discordgo.Disconnect {
	return data.EvtInterface.(*discordgo.Disconnect)
}
func (data *EventData) GuildBanAdd() *discordgo.GuildBanAdd {
	return data.EvtInterface.(*discordgo.GuildBanAdd)
}
func (data *EventData) GuildBanRemove() *discordgo.GuildBanRemove {
	return data.EvtInterface.(*discordgo.GuildBanRemove)
}
func (data *EventData) GuildCreate() *discordgo.GuildCreate {
	return data.EvtInterface.(*discordgo.GuildCreate)
}
func (data *EventData) GuildDelete() *discordgo.GuildDelete {
	return data.EvtInterface.(*discordgo.GuildDelete)
}
func (data *EventData) GuildEmojisUpdate() *discordgo.GuildEmojisUpdate {
	return data.EvtInterface.(*discordgo.GuildEmojisUpdate)
}
func (data *EventData) GuildIntegrationsUpdate() *discordgo.GuildIntegrationsUpdate {
	return data.EvtInterface.(*discordgo.GuildIntegrationsUpdate)
}
func (data *EventData) GuildMemberAdd() *discordgo.GuildMemberAdd {
	return data.EvtInterface.(*discordgo.GuildMemberAdd)
}
func (data *EventData) GuildMemberRemove() *discordgo.GuildMemberRemove {
	return data.EvtInterface.(*discordgo.GuildMemberRemove)
}
func (data *EventData) GuildMemberUpdate() *discordgo.GuildMemberUpdate {
	return data.EvtInterface.(*discordgo.GuildMemberUpdate)
}
func (data *EventData) GuildMembersChunk() *discordgo.GuildMembersChunk {
	return data.EvtInterface.(*discordgo.GuildMembersChunk)
}
func (data *EventData) GuildRoleCreate() *discordgo.GuildRoleCreate {
	return data.EvtInterface.(*discordgo.GuildRoleCreate)
}
func (data *EventData) GuildRoleDelete() *discordgo.GuildRoleDelete {
	return data.EvtInterface.(*discordgo.GuildRoleDelete)
}
func (data *EventData) GuildRoleUpdate() *discordgo.GuildRoleUpdate {
	return data.EvtInterface.(*discordgo.GuildRoleUpdate)
}
func (data *EventData) GuildUpdate() *discordgo.GuildUpdate {
	return data.EvtInterface.(*discordgo.GuildUpdate)
}
func (data *EventData) MessageAck() *discordgo.MessageAck {
	return data.EvtInterface.(*discordgo.MessageAck)
}
func (data *EventData) MessageCreate() *discordgo.MessageCreate {
	return data.EvtInterface.(*discordgo.MessageCreate)
}
func (data *EventData) MessageDelete() *discordgo.MessageDelete {
	return data.EvtInterface.(*discordgo.MessageDelete)
}
func (data *EventData) MessageDeleteBulk() *discordgo.MessageDeleteBulk {
	return data.EvtInterface.(*discordgo.MessageDeleteBulk)
}
func (data *EventData) MessageReactionAdd() *discordgo.MessageReactionAdd {
	return data.EvtInterface.(*discordgo.MessageReactionAdd)
}
func (data *EventData) MessageReactionRemove() *discordgo.MessageReactionRemove {
	return data.EvtInterface.(*discordgo.MessageReactionRemove)
}
func (data *EventData) MessageReactionRemoveAll() *discordgo.MessageReactionRemoveAll {
	return data.EvtInterface.(*discordgo.MessageReactionRemoveAll)
}
func (data *EventData) MessageUpdate() *discordgo.MessageUpdate {
	return data.EvtInterface.(*discordgo.MessageUpdate)
}
func (data *EventData) PresenceUpdate() *discordgo.PresenceUpdate {
	return data.EvtInterface.(*discordgo.PresenceUpdate)
}
func (data *EventData) PresencesReplace() *discordgo.PresencesReplace {
	return data.EvtInterface.(*discordgo.PresencesReplace)
}
func (data *EventData) RateLimit() *discordgo.RateLimit {
	return data.EvtInterface.(*discordgo.RateLimit)
}
func (data *EventData) Ready() *discordgo.Ready {
	return data.EvtInterface.(*discordgo.Ready)
}
func (data *EventData) RelationshipAdd() *discordgo.RelationshipAdd {
	return data.EvtInterface.(*discordgo.RelationshipAdd)
}
func (data *EventData) RelationshipRemove() *discordgo.RelationshipRemove {
	return data.EvtInterface.(*discordgo.RelationshipRemove)
}
func (data *EventData) Resumed() *discordgo.Resumed {
	return data.EvtInterface.(*discordgo.Resumed)
}
func (data *EventData) TypingStart() *discordgo.TypingStart {
	return data.EvtInterface.(*discordgo.TypingStart)
}
func (data *EventData) UserGuildSettingsUpdate() *discordgo.UserGuildSettingsUpdate {
	return data.EvtInterface.(*discordgo.UserGuildSettingsUpdate)
}
func (data *EventData) UserNoteUpdate() *discordgo.UserNoteUpdate {
	return data.EvtInterface.(*discordgo.UserNoteUpdate)
}
func (data *EventData) UserSettingsUpdate() *discordgo.UserSettingsUpdate {
	return data.EvtInterface.(*discordgo.UserSettingsUpdate)
}
func (data *EventData) UserUpdate() *discordgo.UserUpdate {
	return data.EvtInterface.(*discordgo.UserUpdate)
}
func (data *EventData) VoiceServerUpdate() *discordgo.VoiceServerUpdate {
	return data.EvtInterface.(*discordgo.VoiceServerUpdate)
}
func (data *EventData) VoiceStateUpdate() *discordgo.VoiceStateUpdate {
	return data.EvtInterface.(*discordgo.VoiceStateUpdate)
}
func (data *EventData) WebhooksUpdate() *discordgo.WebhooksUpdate {
	return data.EvtInterface.(*discordgo.WebhooksUpdate)
}

func fillEvent(evtData *EventData) {

	switch evtData.EvtInterface.(type) {
	case *discordgo.ChannelCreate:
		evtData.Type = Event(5)
	case *discordgo.ChannelDelete:
		evtData.Type = Event(6)
	case *discordgo.ChannelPinsUpdate:
		evtData.Type = Event(7)
	case *discordgo.ChannelUpdate:
		evtData.Type = Event(8)
	case *discordgo.Connect:
		evtData.Type = Event(9)
	case *discordgo.Disconnect:
		evtData.Type = Event(10)
	case *discordgo.GuildBanAdd:
		evtData.Type = Event(11)
	case *discordgo.GuildBanRemove:
		evtData.Type = Event(12)
	case *discordgo.GuildCreate:
		evtData.Type = Event(13)
	case *discordgo.GuildDelete:
		evtData.Type = Event(14)
	case *discordgo.GuildEmojisUpdate:
		evtData.Type = Event(15)
	case *discordgo.GuildIntegrationsUpdate:
		evtData.Type = Event(16)
	case *discordgo.GuildMemberAdd:
		evtData.Type = Event(17)
	case *discordgo.GuildMemberRemove:
		evtData.Type = Event(18)
	case *discordgo.GuildMemberUpdate:
		evtData.Type = Event(19)
	case *discordgo.GuildMembersChunk:
		evtData.Type = Event(20)
	case *discordgo.GuildRoleCreate:
		evtData.Type = Event(21)
	case *discordgo.GuildRoleDelete:
		evtData.Type = Event(22)
	case *discordgo.GuildRoleUpdate:
		evtData.Type = Event(23)
	case *discordgo.GuildUpdate:
		evtData.Type = Event(24)
	case *discordgo.MessageAck:
		evtData.Type = Event(25)
	case *discordgo.MessageCreate:
		evtData.Type = Event(26)
	case *discordgo.MessageDelete:
		evtData.Type = Event(27)
	case *discordgo.MessageDeleteBulk:
		evtData.Type = Event(28)
	case *discordgo.MessageReactionAdd:
		evtData.Type = Event(29)
	case *discordgo.MessageReactionRemove:
		evtData.Type = Event(30)
	case *discordgo.MessageReactionRemoveAll:
		evtData.Type = Event(31)
	case *discordgo.MessageUpdate:
		evtData.Type = Event(32)
	case *discordgo.PresenceUpdate:
		evtData.Type = Event(33)
	case *discordgo.PresencesReplace:
		evtData.Type = Event(34)
	case *discordgo.RateLimit:
		evtData.Type = Event(35)
	case *discordgo.Ready:
		evtData.Type = Event(36)
	case *discordgo.RelationshipAdd:
		evtData.Type = Event(37)
	case *discordgo.RelationshipRemove:
		evtData.Type = Event(38)
	case *discordgo.Resumed:
		evtData.Type = Event(39)
	case *discordgo.TypingStart:
		evtData.Type = Event(40)
	case *discordgo.UserGuildSettingsUpdate:
		evtData.Type = Event(41)
	case *discordgo.UserNoteUpdate:
		evtData.Type = Event(42)
	case *discordgo.UserSettingsUpdate:
		evtData.Type = Event(43)
	case *discordgo.UserUpdate:
		evtData.Type = Event(44)
	case *discordgo.VoiceServerUpdate:
		evtData.Type = Event(45)
	case *discordgo.VoiceStateUpdate:
		evtData.Type = Event(46)
	case *discordgo.WebhooksUpdate:
		evtData.Type = Event(47)
	default:
		return
	}

	return
}
