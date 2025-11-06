// GENERATED using events_gen.go

// Custom event handlers that adds a redis connection to the handler
// They will also recover from panics

package eventsystem

import (
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type Event int

const (
	EventNewGuild                            Event = 0
	EventAll                                 Event = 1
	EventAllPre                              Event = 2
	EventAllPost                             Event = 3
	EventMemberFetched                       Event = 4
	EventYagShardReady                       Event = 5
	EventYagShardsAdded                      Event = 6
	EventYagShardRemoved                     Event = 7
	EventApplicationCommandCreate            Event = 8
	EventApplicationCommandDelete            Event = 9
	EventApplicationCommandPermissionsUpdate Event = 10
	EventApplicationCommandUpdate            Event = 11
	EventAutoModerationActionExecution       Event = 12
	EventAutoModerationRuleCreate            Event = 13
	EventAutoModerationRuleDelete            Event = 14
	EventAutoModerationRuleUpdate            Event = 15
	EventChannelCreate                       Event = 16
	EventChannelDelete                       Event = 17
	EventChannelPinsUpdate                   Event = 18
	EventChannelTopicUpdate                  Event = 19
	EventChannelUpdate                       Event = 20
	EventConnect                             Event = 21
	EventDisconnect                          Event = 22
	EventEntitlementCreate                   Event = 23
	EventEntitlementDelete                   Event = 24
	EventEntitlementUpdate                   Event = 25
	EventGiftCodeUpdate                      Event = 26
	EventGuildAppliedBoostUpdate             Event = 27
	EventGuildAuditLogEntryCreate            Event = 28
	EventGuildBanAdd                         Event = 29
	EventGuildBanRemove                      Event = 30
	EventGuildCreate                         Event = 31
	EventGuildDelete                         Event = 32
	EventGuildEmojisUpdate                   Event = 33
	EventGuildIntegrationsUpdate             Event = 34
	EventGuildJoinRequestCreate              Event = 35
	EventGuildJoinRequestDelete              Event = 36
	EventGuildJoinRequestUpdate              Event = 37
	EventGuildMemberAdd                      Event = 38
	EventGuildMemberRemove                   Event = 39
	EventGuildMemberUpdate                   Event = 40
	EventGuildMembersChunk                   Event = 41
	EventGuildPowerupEntitlementsCreate      Event = 42
	EventGuildRoleCreate                     Event = 43
	EventGuildRoleDelete                     Event = 44
	EventGuildRoleUpdate                     Event = 45
	EventGuildScheduledEventCreate           Event = 46
	EventGuildScheduledEventDelete           Event = 47
	EventGuildScheduledEventUpdate           Event = 48
	EventGuildScheduledEventUserAdd          Event = 49
	EventGuildScheduledEventUserRemove       Event = 50
	EventGuildSoundboardSoundCreate          Event = 51
	EventGuildSoundboardSoundDelete          Event = 52
	EventGuildSoundboardSoundsUpdate         Event = 53
	EventGuildStickersUpdate                 Event = 54
	EventGuildUpdate                         Event = 55
	EventInteractionCreate                   Event = 56
	EventInviteCreate                        Event = 57
	EventInviteDelete                        Event = 58
	EventMessageAck                          Event = 59
	EventMessageCreate                       Event = 60
	EventMessageDelete                       Event = 61
	EventMessageDeleteBulk                   Event = 62
	EventMessageReactionAdd                  Event = 63
	EventMessageReactionRemove               Event = 64
	EventMessageReactionRemoveAll            Event = 65
	EventMessageReactionRemoveEmoji          Event = 66
	EventMessageUpdate                       Event = 67
	EventPresenceUpdate                      Event = 68
	EventPresencesReplace                    Event = 69
	EventRateLimit                           Event = 70
	EventReady                               Event = 71
	EventRelationshipAdd                     Event = 72
	EventRelationshipRemove                  Event = 73
	EventResumed                             Event = 74
	EventStageInstanceCreate                 Event = 75
	EventStageInstanceDelete                 Event = 76
	EventStageInstanceUpdate                 Event = 77
	EventSubscriptionCreate                  Event = 78
	EventSubscriptionDelete                  Event = 79
	EventSubscriptionUpdate                  Event = 80
	EventThreadCreate                        Event = 81
	EventThreadDelete                        Event = 82
	EventThreadListSync                      Event = 83
	EventThreadMemberUpdate                  Event = 84
	EventThreadMembersUpdate                 Event = 85
	EventThreadUpdate                        Event = 86
	EventTypingStart                         Event = 87
	EventUserGuildSettingsUpdate             Event = 88
	EventUserNoteUpdate                      Event = 89
	EventUserSettingsUpdate                  Event = 90
	EventUserUpdate                          Event = 91
	EventVoiceChannelEffectSend              Event = 92
	EventVoiceChannelStartTimeStatusUpdate   Event = 93
	EventVoiceChannelStatusUpdate            Event = 94
	EventVoiceServerUpdate                   Event = 95
	EventVoiceStateUpdate                    Event = 96
	EventWebhooksUpdate                      Event = 97
)

var EventNames = []string{
	"NewGuild",
	"All",
	"AllPre",
	"AllPost",
	"MemberFetched",
	"YagShardReady",
	"YagShardsAdded",
	"YagShardRemoved",
	"ApplicationCommandCreate",
	"ApplicationCommandDelete",
	"ApplicationCommandPermissionsUpdate",
	"ApplicationCommandUpdate",
	"AutoModerationActionExecution",
	"AutoModerationRuleCreate",
	"AutoModerationRuleDelete",
	"AutoModerationRuleUpdate",
	"ChannelCreate",
	"ChannelDelete",
	"ChannelPinsUpdate",
	"ChannelTopicUpdate",
	"ChannelUpdate",
	"Connect",
	"Disconnect",
	"EntitlementCreate",
	"EntitlementDelete",
	"EntitlementUpdate",
	"GiftCodeUpdate",
	"GuildAppliedBoostUpdate",
	"GuildAuditLogEntryCreate",
	"GuildBanAdd",
	"GuildBanRemove",
	"GuildCreate",
	"GuildDelete",
	"GuildEmojisUpdate",
	"GuildIntegrationsUpdate",
	"GuildJoinRequestCreate",
	"GuildJoinRequestDelete",
	"GuildJoinRequestUpdate",
	"GuildMemberAdd",
	"GuildMemberRemove",
	"GuildMemberUpdate",
	"GuildMembersChunk",
	"GuildPowerupEntitlementsCreate",
	"GuildRoleCreate",
	"GuildRoleDelete",
	"GuildRoleUpdate",
	"GuildScheduledEventCreate",
	"GuildScheduledEventDelete",
	"GuildScheduledEventUpdate",
	"GuildScheduledEventUserAdd",
	"GuildScheduledEventUserRemove",
	"GuildSoundboardSoundCreate",
	"GuildSoundboardSoundDelete",
	"GuildSoundboardSoundsUpdate",
	"GuildStickersUpdate",
	"GuildUpdate",
	"InteractionCreate",
	"InviteCreate",
	"InviteDelete",
	"MessageAck",
	"MessageCreate",
	"MessageDelete",
	"MessageDeleteBulk",
	"MessageReactionAdd",
	"MessageReactionRemove",
	"MessageReactionRemoveAll",
	"MessageReactionRemoveEmoji",
	"MessageUpdate",
	"PresenceUpdate",
	"PresencesReplace",
	"RateLimit",
	"Ready",
	"RelationshipAdd",
	"RelationshipRemove",
	"Resumed",
	"StageInstanceCreate",
	"StageInstanceDelete",
	"StageInstanceUpdate",
	"SubscriptionCreate",
	"SubscriptionDelete",
	"SubscriptionUpdate",
	"ThreadCreate",
	"ThreadDelete",
	"ThreadListSync",
	"ThreadMemberUpdate",
	"ThreadMembersUpdate",
	"ThreadUpdate",
	"TypingStart",
	"UserGuildSettingsUpdate",
	"UserNoteUpdate",
	"UserSettingsUpdate",
	"UserUpdate",
	"VoiceChannelEffectSend",
	"VoiceChannelStartTimeStatusUpdate",
	"VoiceChannelStatusUpdate",
	"VoiceServerUpdate",
	"VoiceStateUpdate",
	"WebhooksUpdate",
}

func (e Event) String() string {
	return EventNames[e]
}

var AllDiscordEvents = []Event{
	EventApplicationCommandCreate,
	EventApplicationCommandDelete,
	EventApplicationCommandPermissionsUpdate,
	EventApplicationCommandUpdate,
	EventAutoModerationActionExecution,
	EventAutoModerationRuleCreate,
	EventAutoModerationRuleDelete,
	EventAutoModerationRuleUpdate,
	EventChannelCreate,
	EventChannelDelete,
	EventChannelPinsUpdate,
	EventChannelTopicUpdate,
	EventChannelUpdate,
	EventConnect,
	EventDisconnect,
	EventEntitlementCreate,
	EventEntitlementDelete,
	EventEntitlementUpdate,
	EventGiftCodeUpdate,
	EventGuildAppliedBoostUpdate,
	EventGuildAuditLogEntryCreate,
	EventGuildBanAdd,
	EventGuildBanRemove,
	EventGuildCreate,
	EventGuildDelete,
	EventGuildEmojisUpdate,
	EventGuildIntegrationsUpdate,
	EventGuildJoinRequestCreate,
	EventGuildJoinRequestDelete,
	EventGuildJoinRequestUpdate,
	EventGuildMemberAdd,
	EventGuildMemberRemove,
	EventGuildMemberUpdate,
	EventGuildMembersChunk,
	EventGuildPowerupEntitlementsCreate,
	EventGuildRoleCreate,
	EventGuildRoleDelete,
	EventGuildRoleUpdate,
	EventGuildScheduledEventCreate,
	EventGuildScheduledEventDelete,
	EventGuildScheduledEventUpdate,
	EventGuildScheduledEventUserAdd,
	EventGuildScheduledEventUserRemove,
	EventGuildSoundboardSoundCreate,
	EventGuildSoundboardSoundDelete,
	EventGuildSoundboardSoundsUpdate,
	EventGuildStickersUpdate,
	EventGuildUpdate,
	EventInteractionCreate,
	EventInviteCreate,
	EventInviteDelete,
	EventMessageAck,
	EventMessageCreate,
	EventMessageDelete,
	EventMessageDeleteBulk,
	EventMessageReactionAdd,
	EventMessageReactionRemove,
	EventMessageReactionRemoveAll,
	EventMessageReactionRemoveEmoji,
	EventMessageUpdate,
	EventPresenceUpdate,
	EventPresencesReplace,
	EventRateLimit,
	EventReady,
	EventRelationshipAdd,
	EventRelationshipRemove,
	EventResumed,
	EventStageInstanceCreate,
	EventStageInstanceDelete,
	EventStageInstanceUpdate,
	EventSubscriptionCreate,
	EventSubscriptionDelete,
	EventSubscriptionUpdate,
	EventThreadCreate,
	EventThreadDelete,
	EventThreadListSync,
	EventThreadMemberUpdate,
	EventThreadMembersUpdate,
	EventThreadUpdate,
	EventTypingStart,
	EventUserGuildSettingsUpdate,
	EventUserNoteUpdate,
	EventUserSettingsUpdate,
	EventUserUpdate,
	EventVoiceChannelEffectSend,
	EventVoiceChannelStartTimeStatusUpdate,
	EventVoiceChannelStatusUpdate,
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
	EventYagShardReady,
	EventYagShardsAdded,
	EventYagShardRemoved,
	EventApplicationCommandCreate,
	EventApplicationCommandDelete,
	EventApplicationCommandPermissionsUpdate,
	EventApplicationCommandUpdate,
	EventAutoModerationActionExecution,
	EventAutoModerationRuleCreate,
	EventAutoModerationRuleDelete,
	EventAutoModerationRuleUpdate,
	EventChannelCreate,
	EventChannelDelete,
	EventChannelPinsUpdate,
	EventChannelTopicUpdate,
	EventChannelUpdate,
	EventConnect,
	EventDisconnect,
	EventEntitlementCreate,
	EventEntitlementDelete,
	EventEntitlementUpdate,
	EventGiftCodeUpdate,
	EventGuildAppliedBoostUpdate,
	EventGuildAuditLogEntryCreate,
	EventGuildBanAdd,
	EventGuildBanRemove,
	EventGuildCreate,
	EventGuildDelete,
	EventGuildEmojisUpdate,
	EventGuildIntegrationsUpdate,
	EventGuildJoinRequestCreate,
	EventGuildJoinRequestDelete,
	EventGuildJoinRequestUpdate,
	EventGuildMemberAdd,
	EventGuildMemberRemove,
	EventGuildMemberUpdate,
	EventGuildMembersChunk,
	EventGuildPowerupEntitlementsCreate,
	EventGuildRoleCreate,
	EventGuildRoleDelete,
	EventGuildRoleUpdate,
	EventGuildScheduledEventCreate,
	EventGuildScheduledEventDelete,
	EventGuildScheduledEventUpdate,
	EventGuildScheduledEventUserAdd,
	EventGuildScheduledEventUserRemove,
	EventGuildSoundboardSoundCreate,
	EventGuildSoundboardSoundDelete,
	EventGuildSoundboardSoundsUpdate,
	EventGuildStickersUpdate,
	EventGuildUpdate,
	EventInteractionCreate,
	EventInviteCreate,
	EventInviteDelete,
	EventMessageAck,
	EventMessageCreate,
	EventMessageDelete,
	EventMessageDeleteBulk,
	EventMessageReactionAdd,
	EventMessageReactionRemove,
	EventMessageReactionRemoveAll,
	EventMessageReactionRemoveEmoji,
	EventMessageUpdate,
	EventPresenceUpdate,
	EventPresencesReplace,
	EventRateLimit,
	EventReady,
	EventRelationshipAdd,
	EventRelationshipRemove,
	EventResumed,
	EventStageInstanceCreate,
	EventStageInstanceDelete,
	EventStageInstanceUpdate,
	EventSubscriptionCreate,
	EventSubscriptionDelete,
	EventSubscriptionUpdate,
	EventThreadCreate,
	EventThreadDelete,
	EventThreadListSync,
	EventThreadMemberUpdate,
	EventThreadMembersUpdate,
	EventThreadUpdate,
	EventTypingStart,
	EventUserGuildSettingsUpdate,
	EventUserNoteUpdate,
	EventUserSettingsUpdate,
	EventUserUpdate,
	EventVoiceChannelEffectSend,
	EventVoiceChannelStartTimeStatusUpdate,
	EventVoiceChannelStatusUpdate,
	EventVoiceServerUpdate,
	EventVoiceStateUpdate,
	EventWebhooksUpdate,
}

var handlers = make([][][]*Handler, 98)

func (data *EventData) ApplicationCommandCreate() *discordgo.ApplicationCommandCreate {
	return data.EvtInterface.(*discordgo.ApplicationCommandCreate)
}
func (data *EventData) ApplicationCommandDelete() *discordgo.ApplicationCommandDelete {
	return data.EvtInterface.(*discordgo.ApplicationCommandDelete)
}
func (data *EventData) ApplicationCommandPermissionsUpdate() *discordgo.ApplicationCommandPermissionsUpdate {
	return data.EvtInterface.(*discordgo.ApplicationCommandPermissionsUpdate)
}
func (data *EventData) ApplicationCommandUpdate() *discordgo.ApplicationCommandUpdate {
	return data.EvtInterface.(*discordgo.ApplicationCommandUpdate)
}
func (data *EventData) AutoModerationActionExecution() *discordgo.AutoModerationActionExecution {
	return data.EvtInterface.(*discordgo.AutoModerationActionExecution)
}
func (data *EventData) AutoModerationRuleCreate() *discordgo.AutoModerationRuleCreate {
	return data.EvtInterface.(*discordgo.AutoModerationRuleCreate)
}
func (data *EventData) AutoModerationRuleDelete() *discordgo.AutoModerationRuleDelete {
	return data.EvtInterface.(*discordgo.AutoModerationRuleDelete)
}
func (data *EventData) AutoModerationRuleUpdate() *discordgo.AutoModerationRuleUpdate {
	return data.EvtInterface.(*discordgo.AutoModerationRuleUpdate)
}
func (data *EventData) ChannelCreate() *discordgo.ChannelCreate {
	return data.EvtInterface.(*discordgo.ChannelCreate)
}
func (data *EventData) ChannelDelete() *discordgo.ChannelDelete {
	return data.EvtInterface.(*discordgo.ChannelDelete)
}
func (data *EventData) ChannelPinsUpdate() *discordgo.ChannelPinsUpdate {
	return data.EvtInterface.(*discordgo.ChannelPinsUpdate)
}
func (data *EventData) ChannelTopicUpdate() *discordgo.ChannelTopicUpdate {
	return data.EvtInterface.(*discordgo.ChannelTopicUpdate)
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
func (data *EventData) EntitlementCreate() *discordgo.EntitlementCreate {
	return data.EvtInterface.(*discordgo.EntitlementCreate)
}
func (data *EventData) EntitlementDelete() *discordgo.EntitlementDelete {
	return data.EvtInterface.(*discordgo.EntitlementDelete)
}
func (data *EventData) EntitlementUpdate() *discordgo.EntitlementUpdate {
	return data.EvtInterface.(*discordgo.EntitlementUpdate)
}
func (data *EventData) GiftCodeUpdate() *discordgo.GiftCodeUpdate {
	return data.EvtInterface.(*discordgo.GiftCodeUpdate)
}
func (data *EventData) GuildAppliedBoostUpdate() *discordgo.GuildAppliedBoostUpdate {
	return data.EvtInterface.(*discordgo.GuildAppliedBoostUpdate)
}
func (data *EventData) GuildAuditLogEntryCreate() *discordgo.GuildAuditLogEntryCreate {
	return data.EvtInterface.(*discordgo.GuildAuditLogEntryCreate)
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
func (data *EventData) GuildJoinRequestCreate() *discordgo.GuildJoinRequestCreate {
	return data.EvtInterface.(*discordgo.GuildJoinRequestCreate)
}
func (data *EventData) GuildJoinRequestDelete() *discordgo.GuildJoinRequestDelete {
	return data.EvtInterface.(*discordgo.GuildJoinRequestDelete)
}
func (data *EventData) GuildJoinRequestUpdate() *discordgo.GuildJoinRequestUpdate {
	return data.EvtInterface.(*discordgo.GuildJoinRequestUpdate)
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
func (data *EventData) GuildPowerupEntitlementsCreate() *discordgo.GuildPowerupEntitlementsCreate {
	return data.EvtInterface.(*discordgo.GuildPowerupEntitlementsCreate)
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
func (data *EventData) GuildScheduledEventCreate() *discordgo.GuildScheduledEventCreate {
	return data.EvtInterface.(*discordgo.GuildScheduledEventCreate)
}
func (data *EventData) GuildScheduledEventDelete() *discordgo.GuildScheduledEventDelete {
	return data.EvtInterface.(*discordgo.GuildScheduledEventDelete)
}
func (data *EventData) GuildScheduledEventUpdate() *discordgo.GuildScheduledEventUpdate {
	return data.EvtInterface.(*discordgo.GuildScheduledEventUpdate)
}
func (data *EventData) GuildScheduledEventUserAdd() *discordgo.GuildScheduledEventUserAdd {
	return data.EvtInterface.(*discordgo.GuildScheduledEventUserAdd)
}
func (data *EventData) GuildScheduledEventUserRemove() *discordgo.GuildScheduledEventUserRemove {
	return data.EvtInterface.(*discordgo.GuildScheduledEventUserRemove)
}
func (data *EventData) GuildSoundboardSoundCreate() *discordgo.GuildSoundboardSoundCreate {
	return data.EvtInterface.(*discordgo.GuildSoundboardSoundCreate)
}
func (data *EventData) GuildSoundboardSoundDelete() *discordgo.GuildSoundboardSoundDelete {
	return data.EvtInterface.(*discordgo.GuildSoundboardSoundDelete)
}
func (data *EventData) GuildSoundboardSoundsUpdate() *discordgo.GuildSoundboardSoundsUpdate {
	return data.EvtInterface.(*discordgo.GuildSoundboardSoundsUpdate)
}
func (data *EventData) GuildStickersUpdate() *discordgo.GuildStickersUpdate {
	return data.EvtInterface.(*discordgo.GuildStickersUpdate)
}
func (data *EventData) GuildUpdate() *discordgo.GuildUpdate {
	return data.EvtInterface.(*discordgo.GuildUpdate)
}
func (data *EventData) InteractionCreate() *discordgo.InteractionCreate {
	return data.EvtInterface.(*discordgo.InteractionCreate)
}
func (data *EventData) InviteCreate() *discordgo.InviteCreate {
	return data.EvtInterface.(*discordgo.InviteCreate)
}
func (data *EventData) InviteDelete() *discordgo.InviteDelete {
	return data.EvtInterface.(*discordgo.InviteDelete)
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
func (data *EventData) MessageReactionRemoveEmoji() *discordgo.MessageReactionRemoveEmoji {
	return data.EvtInterface.(*discordgo.MessageReactionRemoveEmoji)
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
func (data *EventData) StageInstanceCreate() *discordgo.StageInstanceCreate {
	return data.EvtInterface.(*discordgo.StageInstanceCreate)
}
func (data *EventData) StageInstanceDelete() *discordgo.StageInstanceDelete {
	return data.EvtInterface.(*discordgo.StageInstanceDelete)
}
func (data *EventData) StageInstanceUpdate() *discordgo.StageInstanceUpdate {
	return data.EvtInterface.(*discordgo.StageInstanceUpdate)
}
func (data *EventData) SubscriptionCreate() *discordgo.SubscriptionCreate {
	return data.EvtInterface.(*discordgo.SubscriptionCreate)
}
func (data *EventData) SubscriptionDelete() *discordgo.SubscriptionDelete {
	return data.EvtInterface.(*discordgo.SubscriptionDelete)
}
func (data *EventData) SubscriptionUpdate() *discordgo.SubscriptionUpdate {
	return data.EvtInterface.(*discordgo.SubscriptionUpdate)
}
func (data *EventData) ThreadCreate() *discordgo.ThreadCreate {
	return data.EvtInterface.(*discordgo.ThreadCreate)
}
func (data *EventData) ThreadDelete() *discordgo.ThreadDelete {
	return data.EvtInterface.(*discordgo.ThreadDelete)
}
func (data *EventData) ThreadListSync() *discordgo.ThreadListSync {
	return data.EvtInterface.(*discordgo.ThreadListSync)
}
func (data *EventData) ThreadMemberUpdate() *discordgo.ThreadMemberUpdate {
	return data.EvtInterface.(*discordgo.ThreadMemberUpdate)
}
func (data *EventData) ThreadMembersUpdate() *discordgo.ThreadMembersUpdate {
	return data.EvtInterface.(*discordgo.ThreadMembersUpdate)
}
func (data *EventData) ThreadUpdate() *discordgo.ThreadUpdate {
	return data.EvtInterface.(*discordgo.ThreadUpdate)
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
func (data *EventData) VoiceChannelEffectSend() *discordgo.VoiceChannelEffectSend {
	return data.EvtInterface.(*discordgo.VoiceChannelEffectSend)
}
func (data *EventData) VoiceChannelStartTimeStatusUpdate() *discordgo.VoiceChannelStartTimeStatusUpdate {
	return data.EvtInterface.(*discordgo.VoiceChannelStartTimeStatusUpdate)
}
func (data *EventData) VoiceChannelStatusUpdate() *discordgo.VoiceChannelStatusUpdate {
	return data.EvtInterface.(*discordgo.VoiceChannelStatusUpdate)
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
	case *discordgo.ApplicationCommandCreate:
		evtData.Type = Event(8)
	case *discordgo.ApplicationCommandDelete:
		evtData.Type = Event(9)
	case *discordgo.ApplicationCommandPermissionsUpdate:
		evtData.Type = Event(10)
	case *discordgo.ApplicationCommandUpdate:
		evtData.Type = Event(11)
	case *discordgo.AutoModerationActionExecution:
		evtData.Type = Event(12)
	case *discordgo.AutoModerationRuleCreate:
		evtData.Type = Event(13)
	case *discordgo.AutoModerationRuleDelete:
		evtData.Type = Event(14)
	case *discordgo.AutoModerationRuleUpdate:
		evtData.Type = Event(15)
	case *discordgo.ChannelCreate:
		evtData.Type = Event(16)
	case *discordgo.ChannelDelete:
		evtData.Type = Event(17)
	case *discordgo.ChannelPinsUpdate:
		evtData.Type = Event(18)
	case *discordgo.ChannelTopicUpdate:
		evtData.Type = Event(19)
	case *discordgo.ChannelUpdate:
		evtData.Type = Event(20)
	case *discordgo.Connect:
		evtData.Type = Event(21)
	case *discordgo.Disconnect:
		evtData.Type = Event(22)
	case *discordgo.EntitlementCreate:
		evtData.Type = Event(23)
	case *discordgo.EntitlementDelete:
		evtData.Type = Event(24)
	case *discordgo.EntitlementUpdate:
		evtData.Type = Event(25)
	case *discordgo.GiftCodeUpdate:
		evtData.Type = Event(26)
	case *discordgo.GuildAppliedBoostUpdate:
		evtData.Type = Event(27)
	case *discordgo.GuildAuditLogEntryCreate:
		evtData.Type = Event(28)
	case *discordgo.GuildBanAdd:
		evtData.Type = Event(29)
	case *discordgo.GuildBanRemove:
		evtData.Type = Event(30)
	case *discordgo.GuildCreate:
		evtData.Type = Event(31)
	case *discordgo.GuildDelete:
		evtData.Type = Event(32)
	case *discordgo.GuildEmojisUpdate:
		evtData.Type = Event(33)
	case *discordgo.GuildIntegrationsUpdate:
		evtData.Type = Event(34)
	case *discordgo.GuildJoinRequestCreate:
		evtData.Type = Event(35)
	case *discordgo.GuildJoinRequestDelete:
		evtData.Type = Event(36)
	case *discordgo.GuildJoinRequestUpdate:
		evtData.Type = Event(37)
	case *discordgo.GuildMemberAdd:
		evtData.Type = Event(38)
	case *discordgo.GuildMemberRemove:
		evtData.Type = Event(39)
	case *discordgo.GuildMemberUpdate:
		evtData.Type = Event(40)
	case *discordgo.GuildMembersChunk:
		evtData.Type = Event(41)
	case *discordgo.GuildPowerupEntitlementsCreate:
		evtData.Type = Event(42)
	case *discordgo.GuildRoleCreate:
		evtData.Type = Event(43)
	case *discordgo.GuildRoleDelete:
		evtData.Type = Event(44)
	case *discordgo.GuildRoleUpdate:
		evtData.Type = Event(45)
	case *discordgo.GuildScheduledEventCreate:
		evtData.Type = Event(46)
	case *discordgo.GuildScheduledEventDelete:
		evtData.Type = Event(47)
	case *discordgo.GuildScheduledEventUpdate:
		evtData.Type = Event(48)
	case *discordgo.GuildScheduledEventUserAdd:
		evtData.Type = Event(49)
	case *discordgo.GuildScheduledEventUserRemove:
		evtData.Type = Event(50)
	case *discordgo.GuildSoundboardSoundCreate:
		evtData.Type = Event(51)
	case *discordgo.GuildSoundboardSoundDelete:
		evtData.Type = Event(52)
	case *discordgo.GuildSoundboardSoundsUpdate:
		evtData.Type = Event(53)
	case *discordgo.GuildStickersUpdate:
		evtData.Type = Event(54)
	case *discordgo.GuildUpdate:
		evtData.Type = Event(55)
	case *discordgo.InteractionCreate:
		evtData.Type = Event(56)
	case *discordgo.InviteCreate:
		evtData.Type = Event(57)
	case *discordgo.InviteDelete:
		evtData.Type = Event(58)
	case *discordgo.MessageAck:
		evtData.Type = Event(59)
	case *discordgo.MessageCreate:
		evtData.Type = Event(60)
	case *discordgo.MessageDelete:
		evtData.Type = Event(61)
	case *discordgo.MessageDeleteBulk:
		evtData.Type = Event(62)
	case *discordgo.MessageReactionAdd:
		evtData.Type = Event(63)
	case *discordgo.MessageReactionRemove:
		evtData.Type = Event(64)
	case *discordgo.MessageReactionRemoveAll:
		evtData.Type = Event(65)
	case *discordgo.MessageReactionRemoveEmoji:
		evtData.Type = Event(66)
	case *discordgo.MessageUpdate:
		evtData.Type = Event(67)
	case *discordgo.PresenceUpdate:
		evtData.Type = Event(68)
	case *discordgo.PresencesReplace:
		evtData.Type = Event(69)
	case *discordgo.RateLimit:
		evtData.Type = Event(70)
	case *discordgo.Ready:
		evtData.Type = Event(71)
	case *discordgo.RelationshipAdd:
		evtData.Type = Event(72)
	case *discordgo.RelationshipRemove:
		evtData.Type = Event(73)
	case *discordgo.Resumed:
		evtData.Type = Event(74)
	case *discordgo.StageInstanceCreate:
		evtData.Type = Event(75)
	case *discordgo.StageInstanceDelete:
		evtData.Type = Event(76)
	case *discordgo.StageInstanceUpdate:
		evtData.Type = Event(77)
	case *discordgo.SubscriptionCreate:
		evtData.Type = Event(78)
	case *discordgo.SubscriptionDelete:
		evtData.Type = Event(79)
	case *discordgo.SubscriptionUpdate:
		evtData.Type = Event(80)
	case *discordgo.ThreadCreate:
		evtData.Type = Event(81)
	case *discordgo.ThreadDelete:
		evtData.Type = Event(82)
	case *discordgo.ThreadListSync:
		evtData.Type = Event(83)
	case *discordgo.ThreadMemberUpdate:
		evtData.Type = Event(84)
	case *discordgo.ThreadMembersUpdate:
		evtData.Type = Event(85)
	case *discordgo.ThreadUpdate:
		evtData.Type = Event(86)
	case *discordgo.TypingStart:
		evtData.Type = Event(87)
	case *discordgo.UserGuildSettingsUpdate:
		evtData.Type = Event(88)
	case *discordgo.UserNoteUpdate:
		evtData.Type = Event(89)
	case *discordgo.UserSettingsUpdate:
		evtData.Type = Event(90)
	case *discordgo.UserUpdate:
		evtData.Type = Event(91)
	case *discordgo.VoiceChannelEffectSend:
		evtData.Type = Event(92)
	case *discordgo.VoiceChannelStartTimeStatusUpdate:
		evtData.Type = Event(93)
	case *discordgo.VoiceChannelStatusUpdate:
		evtData.Type = Event(94)
	case *discordgo.VoiceServerUpdate:
		evtData.Type = Event(95)
	case *discordgo.VoiceStateUpdate:
		evtData.Type = Event(96)
	case *discordgo.WebhooksUpdate:
		evtData.Type = Event(97)
	default:
		return
	}

	return
}
