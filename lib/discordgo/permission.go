package discordgo

const (
	PermissionCreateInstantInvite     int64 = 0x00000000001                      // (1 << 0)	Allows creation of instant invites	T, V, S
	PermissionKickMembers             int64 = 0x00000000002                      // (1 << 1)	Allows kicking members
	PermissionBanMembers              int64 = 0x00000000004                      // (1 << 2)	Allows banning members
	PermissionAdministrator           int64 = 0x00000000008                      // (1 << 3)	Allows all permissions and bypasses channel permission overwrites
	PermissionManageChannels          int64 = 0x00000000010                      // (1 << 4)	Allows management and editing of channels	T, V, S
	PermissionManageGuild             int64 = 0x00000000020                      // (1 << 5)	Allows management and editing of the guild
	PermissionManageServer            int64 = PermissionManageGuild              // deprecated: use PermissionManageGuild
	PermissionAddReactions            int64 = 0x00000000040                      // (1 << 6)	Allows for the addition of reactions to messages	T
	PermissionViewAuditLog            int64 = 0x00000000080                      // (1 << 7)	Allows for viewing of audit logs
	PermissionViewAuditLogs           int64 = PermissionViewAuditLog             // deprecated: use PermissionViewAuditLog
	PermissionPrioritySpeaker         int64 = 0x00000000100                      // (1 << 8)	Allows for using priority speaker in a voice channel	V
	PermissionStream                  int64 = 0x00000000200                      // (1 << 9)	Allows the user to go live	V
	PermissionViewChannel             int64 = 0x00000000400                      // (1 << 10)	Allows guild members to view a channel, which includes reading messages in text channels	T, V, S
	PermissionReadMessages            int64 = PermissionViewChannel              // deprecated: use PermissionViewChannel
	PermissionSendMessages            int64 = 0x00000000800                      // (1 << 11)	Allows for sending messages in a channel	T
	PermissionSendTTSMessages         int64 = 0x00000001000                      // (1 << 12)	Allows for sending of /tts messages	T
	PermissionManageMessages          int64 = 0x00000002000                      // (1 << 13)	Allows for deletion of other users messages	T
	PermissionEmbedLinks              int64 = 0x00000004000                      // (1 << 14)	Links sent by users with this permission will be auto-embedded	T
	PermissionAttachFiles             int64 = 0x00000008000                      // (1 << 15)	Allows for uploading images and files	T
	PermissionReadMessageHistory      int64 = 0x00000010000                      // (1 << 16)	Allows for reading of message history	T
	PermissionMentionEveryone         int64 = 0x00000020000                      // (1 << 17)	Allows for using the @everyone tag to notify all users in a channel, and the @here tag to notify all online users in a channel	T
	PermissionUseExternalEmojis       int64 = 0x00000040000                      // (1 << 18)	Allows the usage of custom emojis from other servers	T
	PermissionViewGuildInsights       int64 = 0x00000080000                      // (1 << 19)	Allows for viewing guild insights
	PermissionVoiceConnect            int64 = 0x00000100000                      // (1 << 20)	Allows for joining of a voice channel	V, S
	PermissionVoiceSpeak              int64 = 0x00000200000                      // (1 << 21)	Allows for speaking in a voice channel	V
	PermissionVoiceMuteMembers        int64 = 0x00000400000                      // (1 << 22)	Allows for muting members in a voice channel	V, S
	PermissionVoiceDeafenMembers      int64 = 0x00000800000                      // (1 << 23)	Allows for deafening of members in a voice channel	V, S
	PermissionVoiceMoveMembers        int64 = 0x00001000000                      // (1 << 24)	Allows for moving of members between voice channels	V, S
	PermissionVoiceUseVAD             int64 = 0x00002000000                      // (1 << 25)	Allows for using voice-activity-detection in a voice channel	V
	PermissionChangeNickname          int64 = 0x00004000000                      // (1 << 26)	Allows for modification of own nickname
	PermissionManageNicknames         int64 = 0x00008000000                      // (1 << 27)	Allows for modification of other users nicknames
	PermissionManageRoles             int64 = 0x00010000000                      // (1 << 28)	Allows management and editing of roles	T, V, S
	PermissionManageWebhooks          int64 = 0x00020000000                      // (1 << 29)	Allows management and editing of webhooks	T
	PermissionManageEmojisAndStickers int64 = 0x00040000000                      // (1 << 30)	Allows management and editing of emojis and stickers
	PermissionManageEmojis            int64 = PermissionManageEmojisAndStickers  // deprecated: use PermissionManageEmojisAndStickers
	PermissionUseSlashCommands        int64 = PermissionUseApplicationCommands   // deprecated: use PermissionUseApplicationCommands
	PermissionUseApplicationCommands  int64 = 0x00080000000                      // (1 << 31)	Allows members to use slash commands in text channels	T
	PermissionRequestToSpeak          int64 = 0x00100000000                      // (1 << 32)	Allows for requesting to speak in stage channels. (This permission is under active development and may be changed or removed.)	S
	PermissionManageEvents            int64 = 0x00200000000                      // (1 << 33)   Allows for creating, editing, and deleting scheduled events	V, S
	PermissionManageThreads           int64 = 0x00400000000                      // (1 << 34)	Allows for deleting and archiving threads, and viewing all private threads	T
	PermissionUsePublicThreads        int64 = 0x00800000000                      // (1 << 35)	Allows for creating and participating in threads	T
	PermissionUsePrivateThreads       int64 = 0x01000000000                      // (1 << 36)	Allows for creating and participating in private threads	T
	PermissionUseExternalStickers     int64 = 0x02000000000                      // (1 << 37)	Allows the usage of custom stickers from other servers	T
	PermissionSendMessagesInThreads   int64 = 0x04000000000                      // (1 << 38)   Allows for sending messages in threads	T
	PermissionUseEmbeddedActivities   int64 = 0x08000000000                      // (1 << 39)   Allows for using Activities (applications with the EMBEDDED flag) in a voice channel	V
	PermissionModerateMembers         int64 = 0x10000000000                      // (1 << 40)   Allows for timing out users to prevent them from sending or reacting to messages in chat and threads, and from speaking in voice and stage channels

	// all bits set except the leftmost to avoid using negative numbers in case discord dosen't handle it
	PermissionAll int64 = int64(^uint64(0) >> 1)
)
