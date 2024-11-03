package discordgo

const (
	PermissionCreateInstantInvite              int64 = 1 << 0  // Allows creation of instant invites	T, V, S
	PermissionKickMembers                      int64 = 1 << 1  // Allows kicking members
	PermissionBanMembers                       int64 = 1 << 2  // Allows banning members
	PermissionAdministrator                    int64 = 1 << 3  // Allows all permissions and bypasses channel permission overwrites
	PermissionManageChannels                   int64 = 1 << 4  // Allows management and editing of channels	T, V, S
	PermissionManageGuild                      int64 = 1 << 5  // Allows management and editing of the guild
	PermissionAddReactions                     int64 = 1 << 6  // Allows for the addition of reactions to messages	T
	PermissionViewAuditLog                     int64 = 1 << 7  // Allows for viewing of audit logs
	PermissionPrioritySpeaker                  int64 = 1 << 8  // Allows for using priority speaker in a voice channel	V
	PermissionStream                           int64 = 1 << 9  // Allows the user to go live	V
	PermissionViewChannel                      int64 = 1 << 10 // Allows guild members to view a channel, which includes reading messages in text channels	T, V, S
	PermissionSendMessages                     int64 = 1 << 11 // Allows for sending messages in a channel	T
	PermissionSendTTSMessages                  int64 = 1 << 12 // Allows for sending of /tts messages	T
	PermissionManageMessages                   int64 = 1 << 13 // Allows for deletion of other users messages	T
	PermissionEmbedLinks                       int64 = 1 << 14 // Links sent by users with this permission will be auto-embedded	T
	PermissionAttachFiles                      int64 = 1 << 15 // Allows for uploading images and files	T
	PermissionReadMessageHistory               int64 = 1 << 16 // Allows for reading of message history	T
	PermissionMentionEveryone                  int64 = 1 << 17 // Allows for using the @everyone tag to notify all users in a channel, and the @here tag to notify all online users in a channel	T
	PermissionUseExternalEmojis                int64 = 1 << 18 // Allows the usage of custom emojis from other servers	T
	PermissionViewGuildInsights                int64 = 1 << 19 // Allows for viewing guild insights
	PermissionVoiceConnect                     int64 = 1 << 20 // Allows for joining of a voice channel	V, S
	PermissionVoiceSpeak                       int64 = 1 << 21 // Allows for speaking in a voice channel	V
	PermissionVoiceMuteMembers                 int64 = 1 << 22 // Allows for muting members in a voice channel	V, S
	PermissionVoiceDeafenMembers               int64 = 1 << 23 // Allows for deafening of members in a voice channel	V, S
	PermissionVoiceMoveMembers                 int64 = 1 << 24 // Allows for moving of members between voice channels	V, S
	PermissionVoiceUseVAD                      int64 = 1 << 25 // Allows for using voice-activity-detection in a voice channel	V
	PermissionChangeNickname                   int64 = 1 << 26 // Allows for modification of own nickname
	PermissionManageNicknames                  int64 = 1 << 27 // Allows for modification of other users nicknames
	PermissionManageRoles                      int64 = 1 << 28 // Allows management and editing of roles	T, V, S
	PermissionManageWebhooks                   int64 = 1 << 29 // Allows management and editing of webhooks	T
	PermissionManageGuildExpressions           int64 = 1 << 30 // Allows management and editing of emojis and stickers
	PermissionUseApplicationCommands           int64 = 1 << 31 // Allows members to use slash commands in text channels	T
	PermissionRequestToSpeak                   int64 = 1 << 32 // Allows for requesting to speak in stage channels. (This permission is under active development and may be changed or removed.)	S
	PermissionManageEvents                     int64 = 1 << 33 // Allows for creating, editing, and deleting scheduled events	V, S
	PermissionManageThreads                    int64 = 1 << 34 // Allows for deleting and archiving threads, and viewing all private threads	T
	PermissionUsePublicThreads                 int64 = 1 << 35 // Allows for creating and participating in threads	T
	PermissionUsePrivateThreads                int64 = 1 << 36 // Allows for creating and participating in private threads	T
	PermissionUseExternalStickers              int64 = 1 << 37 // Allows the usage of custom stickers from other servers	T
	PermissionSendMessagesInThreads            int64 = 1 << 38 // Allows for sending messages in threads	T
	PermissionUseEmbeddedActivities            int64 = 1 << 39 // Allows for using Activities (applications with the EMBEDDED flag) in a voice channel	V
	PermissionModerateMembers                  int64 = 1 << 40 // Allows for timing out users to prevent them from sending or reacting to messages in chat and threads, and from speaking in voice and stage channels
	PermissionViewCreatorMonetizationAnalytics int64 = 1 << 41 // Allows for viewing role subscription insights
	PermissionUseSoundboard                    int64 = 1 << 42 // Allows for using soundboard in a voice channel 	V
	PermissionCreateGuildExpressions           int64 = 1 << 43 // Allows for creating emojis, stickers, and soundboard sounds, and editing and deleting those created by the current user
	PermissionCreateEvents                     int64 = 1 << 44 // Allows for creating scheduled events, and editing and deleting those created by the current user
	PermissionUseExternalSounds                int64 = 1 << 45 // Allows the usage of custom soundboard sounds from other servers 	V
	PermissionSendVoiceMessages                int64 = 1 << 46 // Allows sending voice messages 	T, V, S
	PermissionSendPolls                        int64 = 1 << 49 // Allows sending polls 	T, V, S
)

// all bits set except the leftmost to avoid using negative numbers in case discord doesn't handle it
const PermissionAll int64 = int64(^uint64(0) >> 1)

//go:generate go run tools/cmd/permnames/main.go
var AllPermissions = []int64{
	PermissionAdministrator,
	PermissionManageGuild,
	PermissionViewGuildInsights,
	PermissionViewCreatorMonetizationAnalytics,

	PermissionViewChannel,
	PermissionSendMessages,
	PermissionSendTTSMessages,
	PermissionManageMessages,
	PermissionEmbedLinks,
	PermissionAttachFiles,
	PermissionReadMessageHistory,
	PermissionMentionEveryone,
	PermissionUseExternalEmojis,
	PermissionUseExternalStickers,
	PermissionUseApplicationCommands,
	PermissionUseEmbeddedActivities,
	PermissionUseSoundboard,
	PermissionUseExternalSounds,
	PermissionSendVoiceMessages,
	PermissionSendPolls,

	PermissionVoiceConnect,
	PermissionVoiceSpeak,
	PermissionVoiceMuteMembers,
	PermissionVoiceDeafenMembers,
	PermissionVoiceMoveMembers,
	PermissionVoiceUseVAD,
	PermissionPrioritySpeaker,
	PermissionRequestToSpeak,
	PermissionStream,

	PermissionChangeNickname,
	PermissionManageNicknames,
	PermissionManageRoles,
	PermissionManageWebhooks,
	PermissionManageGuildExpressions,

	PermissionCreateInstantInvite,
	PermissionModerateMembers,
	PermissionKickMembers,
	PermissionBanMembers,
	PermissionManageChannels,
	PermissionManageEvents,
	PermissionCreateGuildExpressions,
	PermissionCreateEvents,

	PermissionManageThreads,
	PermissionUsePublicThreads,
	PermissionUsePrivateThreads,
	PermissionSendMessagesInThreads,

	PermissionAddReactions,
	PermissionViewAuditLog,
}
