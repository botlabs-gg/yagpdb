package common

type ContextKey int

const (
	ContextKeyRedis ContextKey = iota
	ContextKeyDiscordSession
	ContextKeyTemplateData
	ContextKeyUser
	ContextKeyGuilds
	ContextKeyCurrentGuild
	ContextKeyCurrentUserGuild
	ContextKeyGuildChannels
	ContextKeyGuildRoles
	ContextKeyParsedForm
	ContextKeyFormOk
	ContextKeyBotMember
	ContextKeyBotPermissions
	ContextKeyHighestBotRole
	ContextKeyLogger
)
