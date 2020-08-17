package common

type ContextKey int

const (
	ContextKeyRedis ContextKey = iota
	ContextKeyDiscordSession
	ContextKeyTemplateData
	ContextKeyUser
	ContextKeyCurrentGuild
	ContextKeyGuildRoles
	ContextKeyParsedForm
	ContextKeyFormOk
	ContextKeyBotMember
	ContextKeyBotPermissions
	ContextKeyHighestBotRole
	ContextKeyLogger
	ContextKeyIsPartial
	ContextKeyUserMember
	ContextKeyCoreConfig
	ContextKeyMemberPermissions
	ContextKeyIsAdmin
	ContextKeyIsReadOnly
)
