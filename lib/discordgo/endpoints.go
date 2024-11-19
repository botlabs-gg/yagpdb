// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains variables for all known Discord end points.  All functions
// throughout the Discordgo package use these variables for all connections
// to Discord.  These are all exported and you may modify them if needed.

package discordgo

import "strconv"

// APIVersion is the Discord API version used for the REST and Websocket API.
var APIVersion = "10"

// Known Discord API Endpoints.
var (
	EndpointStatus     string
	EndpointSm         string
	EndpointSmActive   string
	EndpointSmUpcoming string

	EndpointDiscord    string
	EndpointAPI        string
	EndpointGuilds     string
	EndpointChannels   string
	EndpointUsers      string
	EndpointGateway    string
	EndpointGatewayBot string
	EndpointWebhooks   string
	EndpointStickers   string

	EndpointCDN             string
	EndpointCDNAttachments  string
	EndpointCDNAvatars      string
	EndpointCDNGuilds       string
	EndpointCDNIcons        string
	EndpointCDNSplashes     string
	EndpointCDNChannelIcons string
	EndpointCDNBanners      string

	EndpointAuth           string
	EndpointLogin          string
	EndpointLogout         string
	EndpointVerify         string
	EndpointVerifyResend   string
	EndpointForgotPassword string
	EndpointResetPassword  string
	EndpointRegister       string

	EndpointVoice        string
	EndpointVoiceRegions string
	EndpointVoiceIce     string

	EndpointTutorial           string
	EndpointTutorialIndicators string

	EndpointTrack        string
	EndpointSso          string
	EndpointReport       string
	EndpointIntegrations string

	EndpointUser               = func(uID string) string { return "" }
	EndpointUserAvatar         = func(uID int64, aID string) string { return "" }
	EndpointUserAvatarAnimated = func(uID int64, aID string) string { return "" }
	EndpointUserSettings       = func(uID string) string { return "" }
	EndpointUserGuilds         = func(uID string) string { return "" }
	EndpointUserGuild          = func(uID string, gID int64) string { return "" }
	EndpointUserGuildSettings  = func(uID string, gID int64) string { return "" }
	EndpointUserChannels       = func(uID string) string { return "" }
	EndpointUserDevices        = func(uID string) string { return "" }
	EndpointUserConnections    = func(uID string) string { return "" }
	EndpointUserNotes          = func(uID int64) string { return "" }

	EndpointGuild                     = func(gID int64) string { return "" }
	EndpointGuildChannels             = func(gID int64) string { return "" }
	EndpointGuildMembers              = func(gID int64) string { return "" }
	EndpointGuildMember               = func(gID int64, uID int64) string { return "" }
	EndpointGuildMemberAvatar         = func(gID, uID int64, aID string) string { return "" }
	EndpointGuildMemberAvatarAnimated = func(gID, uID int64, aID string) string { return "" }
	EndpointGuildMemberMe             = func(gID int64) string { return "" }
	EndpointGuildMemberVoiceState     = func(gID, uID int64) string { return "" }
	EndpointGuildMemberRole           = func(gID, uID, rID int64) string {
		return ""
	}
	EndpointGuildBans            = func(gID int64) string { return "" }
	EndpointGuildBan             = func(gID, uID int64) string { return "" }
	EndpointGuildIntegrations    = func(gID int64) string { return "" }
	EndpointGuildIntegration     = func(gID, iID int64) string { return "" }
	EndpointGuildIntegrationSync = func(gID, iID int64) string {
		return ""
	}
	EndpointGuildRoles          = func(gID int64) string { return "" }
	EndpointGuildRole           = func(gID, rID int64) string { return "" }
	EndpointGuildInvites        = func(gID int64) string { return "" }
	EndpointGuildEmbed          = func(gID int64) string { return "" }
	EndpointGuildPrune          = func(gID int64) string { return "" }
	EndpointGuildIcon           = func(gID int64, hash string) string { return "" }
	EndpointGuildIconAnimated   = func(gID int64, hash string) string { return "" }
	EndpointGuildSplash         = func(gID int64, hash string) string { return "" }
	EndpointGuildWebhooks       = func(gID int64) string { return "" }
	EndpointGuildAuditLogs      = func(gID int64) string { return "" }
	EndpointGuildEmojis         = func(gID int64) string { return "" }
	EndpointGuildEmoji          = func(gID, eID int64) string { return "" }
	EndpointGuildBanner         = func(gID int64, hash string) string { return "" }
	EndpointGuildBannerAnimated = func(gID int64, hash string) string { return "" }
	EndpointGuildThreads        = func(gID int64) string { return "" }
	EndpointGuildActiveThreads  = func(gID int64) string { return "" }
	EndpointGuildStickers       = func(gID int64) string { return "" }
	EndpointGuildSticker        = func(gID, sID int64) string { return "" }

	EndpointChannel                             = func(cID int64) string { return "" }
	EndpointChannelThreads                      = func(cID int64) string { return "" }
	EndpointChannelActiveThreads                = func(cID int64) string { return "" }
	EndpointChannelPublicArchivedThreads        = func(cID int64) string { return "" }
	EndpointChannelPrivateArchivedThreads       = func(cID int64) string { return "" }
	EndpointChannelJoinedPrivateArchivedThreads = func(cID int64) string { return "" }
	EndpointChannelPermissions                  = func(cID int64) string { return "" }
	EndpointChannelPermission                   = func(cID, tID int64) string { return "" }
	EndpointChannelInvites                      = func(cID int64) string { return "" }
	EndpointChannelTyping                       = func(cID int64) string { return "" }
	EndpointChannelMessages                     = func(cID int64) string { return "" }
	EndpointChannelMessage                      = func(cID, mID int64) string { return "" }
	EndpointChannelMessageAck                   = func(cID, mID int64) string { return "" }
	EndpointChannelMessagesBulkDelete           = func(cID int64) string { return "" }
	EndpointChannelMessagesPins                 = func(cID int64) string { return "" }
	EndpointChannelMessagePin                   = func(cID, mID int64) string { return "" }
	EndpointChannelMessageCrosspost             = func(cID, mID int64) string { return "" }
	EndpointChannelMessageThread                = func(cID, mID int64) string { return "" }
	EndpointThreadMembers                       = func(tID int64) string { return "" }
	EndpointThreadMember                        = func(tID int64, mID string) string { return "" }

	EndpointGroupIcon = func(cID int64, hash string) string { return "" }

	EndpointSticker            = func(sID int64) string { return "" }
	EndpointNitroStickersPacks string

	EndpointChannelWebhooks = func(cID int64) string { return "" }
	EndpointWebhook         = func(wID int64) string { return "" }
	EndpointWebhookToken    = func(wID int64, token string) string { return "" }
	EndpointWebhookMessage  = func(wID int64, token, mID string) string { return "" }

	EndpointDefaultUserAvatar = func(index int) string { return "" }

	EndpointMessageReactionsAll = func(cID, mID int64) string { return "" }
	EndpointMessageReactions    = func(cID, mID int64, emoji EmojiName) string {
		return ""
	}
	EndpointMessageReaction = func(cID, mID int64, emoji EmojiName, uID string) string {
		return ""
	}

	EndpointRelationships       = func() string { return "" }
	EndpointRelationship        = func(uID int64) string { return "" }
	EndpointRelationshipsMutual = func(uID int64) string { return "" }

	EndpointGuildCreate = ""

	EndpointInvite = func(iID string) string { return "" }

	EndpointIntegrationsJoin = func(iID string) string { return "" }

	EndpointEmoji         = func(eID int64) string { return "" }
	EndpointEmojiAnimated = func(eID int64) string { return "" }

	EndpointOauth2          = ""
	EndpointApplications    = ""
	EndpointApplication     = func(aID int64) string { return "" }
	EndpointApplicationMe   = ""
	EndpointApplicationsBot = func(aID int64) string { return "" }

	EndpointApplicationNonOauth2 = func(aID int64) string { return "" }
	EndpointApplicationCommands  = func(aID int64) string { return "" }
	EndpointApplicationCommand   = func(aID int64, cmdID int64) string {
		return ""
	}

	EndpointApplicationGuildCommands = func(aID int64, gID int64) string {
		return ""
	}

	EndpointApplicationGuildCommand = func(aID int64, gID int64, cmdID int64) string {
		return ""
	}

	EndpointApplicationGuildCommandsPermissions = func(aID int64, gID int64) string {
		return ""
	}

	EndpointApplicationGuildCommandPermissions = func(aID int64, gID int64, cmdID int64) string {
		return ""
	}

	EndpointInteractions        = ""
	EndpointInteractionCallback = func(interactionID int64, token string) string {
		return ""
	}
	EndpointWebhookInteraction = func(applicationID int64, token string) string {
		return ""
	}
	EndpointInteractionOriginalMessage = func(applicationID int64, token string) string {
		return ""
	}
	EndpointInteractionFollowupMessage = func(applicationID int64, token string, messageID int64) string {
		return ""
	}
	EndpointSKUs = func(applicationID int64) string {
		return ""
	}
	EndpointEntitlements = func(applicationID int64) string {
		return ""
	}
	EndpointEntitlement = func(applicationID, entitlementID int64) string {
		return ""
	}
	EndpointEntitlementConsume = func(applicationID, entitlementID int64) string {
		return ""
	}
	EndpointSKUSubscriptions = func(skuID int64) string {
		return ""
	}
	EndpointSKUSubscription = func(skuID, subscriptionID int64) string {
		return ""
	}
)

func CreateEndpoints(base string) {
	EndpointStatus = "https://status.discord.com/api/v2/"
	EndpointSm = EndpointStatus + "scheduled-maintenances/"
	EndpointSmActive = EndpointSm + "active.json"
	EndpointSmUpcoming = EndpointSm + "upcoming.json"

	EndpointDiscord = base
	EndpointAPI = EndpointDiscord + "api/v" + APIVersion + "/"
	EndpointGuilds = EndpointAPI + "guilds/"
	EndpointChannels = EndpointAPI + "channels/"
	EndpointUsers = EndpointAPI + "users/"
	EndpointGateway = EndpointAPI + "gateway"
	EndpointGatewayBot = EndpointGateway + "/bot"
	EndpointWebhooks = EndpointAPI + "webhooks/"
	EndpointStickers = EndpointAPI + "stickers/"

	EndpointCDN = "https://cdn.discordapp.com/"
	EndpointCDNAttachments = EndpointCDN + "attachments/"
	EndpointCDNAvatars = EndpointCDN + "avatars/"
	EndpointCDNGuilds = EndpointCDN + "guilds/"
	EndpointCDNIcons = EndpointCDN + "icons/"
	EndpointCDNSplashes = EndpointCDN + "splashes/"
	EndpointCDNChannelIcons = EndpointCDN + "channel-icons/"
	EndpointCDNBanners = EndpointCDN + "banners/"

	EndpointAuth = EndpointAPI + "auth/"
	EndpointLogin = EndpointAuth + "login"
	EndpointLogout = EndpointAuth + "logout"
	EndpointVerify = EndpointAuth + "verify"
	EndpointVerifyResend = EndpointAuth + "verify/resend"
	EndpointForgotPassword = EndpointAuth + "forgot"
	EndpointResetPassword = EndpointAuth + "reset"
	EndpointRegister = EndpointAuth + "register"

	EndpointVoice = EndpointAPI + "/voice/"
	EndpointVoiceRegions = EndpointVoice + "regions"
	EndpointVoiceIce = EndpointVoice + "ice"

	EndpointTutorial = EndpointAPI + "tutorial/"
	EndpointTutorialIndicators = EndpointTutorial + "indicators"

	EndpointTrack = EndpointAPI + "track"
	EndpointSso = EndpointAPI + "sso"
	EndpointReport = EndpointAPI + "report"
	EndpointIntegrations = EndpointAPI + "integrations"

	EndpointUser = func(uID string) string { return EndpointUsers + uID }
	EndpointUserAvatar = func(uID int64, aID string) string { return EndpointCDNAvatars + StrID(uID) + "/" + aID + ".png" }
	EndpointUserAvatarAnimated = func(uID int64, aID string) string { return EndpointCDNAvatars + StrID(uID) + "/" + aID + ".gif" }
	EndpointUserSettings = func(uID string) string { return EndpointUsers + uID + "/settings" }
	EndpointUserGuilds = func(uID string) string { return EndpointUsers + uID + "/guilds" }
	EndpointUserGuild = func(uID string, gID int64) string { return EndpointUsers + uID + "/guilds/" + StrID(gID) }
	EndpointUserGuildSettings = func(uID string, gID int64) string { return EndpointUsers + uID + "/guilds/" + StrID(gID) + "/settings" }
	EndpointUserChannels = func(uID string) string { return EndpointUsers + uID + "/channels" }
	EndpointUserDevices = func(uID string) string { return EndpointUsers + uID + "/devices" }
	EndpointUserConnections = func(uID string) string { return EndpointUsers + uID + "/connections" }
	EndpointUserNotes = func(uID int64) string { return EndpointUsers + "@me/notes/" + StrID(uID) }

	EndpointGuild = func(gID int64) string { return EndpointGuilds + StrID(gID) }
	EndpointGuildChannels = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/channels" }
	EndpointGuildMembers = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/members" }
	EndpointGuildMember = func(gID int64, uID int64) string { return EndpointGuilds + StrID(gID) + "/members/" + StrID(uID) }
	EndpointGuildMemberAvatar = func(gID int64, uID int64, aID string) string {
		return EndpointCDNGuilds + StrID(gID) + "/users/" + StrID(uID) + "/avatars/" + aID + ".png"
	}
	EndpointGuildMemberAvatarAnimated = func(gID int64, uID int64, aID string) string {
		return EndpointCDNGuilds + StrID(gID) + "/users/" + StrID(uID) + "/avatars/" + aID + ".gif"
	}
	EndpointGuildMemberMe = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/members/@me" }
	EndpointGuildMemberVoiceState = func(gID, uID int64) string { return EndpointGuilds + StrID(gID) + "/voice-states/" + StrID(uID) }
	EndpointGuildMemberRole = func(gID, uID, rID int64) string {
		return EndpointGuilds + StrID(gID) + "/members/" + StrID(uID) + "/roles/" + StrID(rID)
	}
	EndpointGuildBans = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/bans" }
	EndpointGuildBan = func(gID, uID int64) string { return EndpointGuilds + StrID(gID) + "/bans/" + StrID(uID) }
	EndpointGuildIntegrations = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/integrations" }
	EndpointGuildIntegration = func(gID, iID int64) string { return EndpointGuilds + StrID(gID) + "/integrations/" + StrID(iID) }
	EndpointGuildIntegrationSync = func(gID, iID int64) string {
		return EndpointGuilds + StrID(gID) + "/integrations/" + StrID(iID) + "/sync"
	}
	EndpointGuildRoles = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/roles" }
	EndpointGuildRole = func(gID, rID int64) string { return EndpointGuilds + StrID(gID) + "/roles/" + StrID(rID) }
	EndpointGuildInvites = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/invites" }
	EndpointGuildEmbed = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/embed" }
	EndpointGuildPrune = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/prune" }
	EndpointGuildIcon = func(gID int64, hash string) string { return EndpointCDNIcons + StrID(gID) + "/" + hash + ".png" }
	EndpointGuildIconAnimated = func(gID int64, hash string) string { return EndpointCDNIcons + StrID(gID) + "/" + hash + ".gif" }
	EndpointGuildSplash = func(gID int64, hash string) string { return EndpointCDNSplashes + StrID(gID) + "/" + hash + ".png" }
	EndpointGuildWebhooks = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/webhooks" }
	EndpointGuildAuditLogs = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/audit-logs" }
	EndpointGuildEmojis = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/emojis" }
	EndpointGuildEmoji = func(gID, eID int64) string { return EndpointGuilds + StrID(gID) + "/emojis/" + StrID(eID) }
	EndpointGuildBanner = func(gID int64, hash string) string { return EndpointCDNBanners + StrID(gID) + "/" + hash + ".png" }
	EndpointGuildBannerAnimated = func(gID int64, hash string) string { return EndpointCDNBanners + StrID(gID) + "/" + hash + ".gif" }
	EndpointGuildThreads = func(gID int64) string { return EndpointGuild(gID) + "/threads" }
	EndpointGuildActiveThreads = func(gID int64) string { return EndpointGuildThreads(gID) + "/active" }
	EndpointGuildStickers = func(gID int64) string { return EndpointGuilds + StrID(gID) + "/stickers" }
	EndpointGuildSticker = func(gID, sID int64) string { return EndpointGuilds + StrID(gID) + "/stickers/" + StrID(sID) }

	EndpointChannel = func(cID int64) string { return EndpointChannels + StrID(cID) }
	EndpointChannelThreads = func(cID int64) string { return EndpointChannel(cID) + "/threads" }
	EndpointChannelActiveThreads = func(cID int64) string { return EndpointChannelThreads(cID) + "/active" }
	EndpointChannelPublicArchivedThreads = func(cID int64) string { return EndpointChannelThreads(cID) + "/archived/public" }
	EndpointChannelPrivateArchivedThreads = func(cID int64) string { return EndpointChannelThreads(cID) + "/archived/private" }
	EndpointChannelJoinedPrivateArchivedThreads = func(cID int64) string { return EndpointChannel(cID) + "/users/@me/threads/archived/private" }
	EndpointChannelPermissions = func(cID int64) string { return EndpointChannels + StrID(cID) + "/permissions" }
	EndpointChannelPermission = func(cID, tID int64) string { return EndpointChannels + StrID(cID) + "/permissions/" + StrID(tID) }
	EndpointChannelInvites = func(cID int64) string { return EndpointChannels + StrID(cID) + "/invites" }
	EndpointChannelTyping = func(cID int64) string { return EndpointChannels + StrID(cID) + "/typing" }
	EndpointChannelMessages = func(cID int64) string { return EndpointChannels + StrID(cID) + "/messages" }
	EndpointChannelMessage = func(cID, mID int64) string { return EndpointChannels + StrID(cID) + "/messages/" + StrID(mID) }
	EndpointChannelMessageAck = func(cID, mID int64) string { return EndpointChannels + StrID(cID) + "/messages/" + StrID(mID) + "/ack" }
	EndpointChannelMessagesBulkDelete = func(cID int64) string { return EndpointChannel(cID) + "/messages/bulk-delete" }
	EndpointChannelMessagesPins = func(cID int64) string { return EndpointChannel(cID) + "/pins" }
	EndpointChannelMessagePin = func(cID, mID int64) string { return EndpointChannel(cID) + "/pins/" + StrID(mID) }
	EndpointChannelMessageCrosspost = func(cID, mID int64) string { return EndpointChannel(cID) + "/messages/" + StrID(mID) + "/crosspost" }
	EndpointChannelMessageThread = func(cID, mID int64) string { return EndpointChannelMessage(cID, mID) + "/threads" }
	EndpointThreadMembers = func(tID int64) string { return EndpointChannel(tID) + "/thread-members" }
	EndpointThreadMember = func(tID int64, mID string) string { return EndpointThreadMembers(tID) + "/" + mID }

	EndpointGroupIcon = func(cID int64, hash string) string { return EndpointCDNChannelIcons + StrID(cID) + "/" + hash + ".png" }

	EndpointSticker = func(sID int64) string { return EndpointStickers + StrID(sID) }
	EndpointNitroStickersPacks = EndpointAPI + "/sticker-packs"

	EndpointChannelWebhooks = func(cID int64) string { return EndpointChannel(cID) + "/webhooks" }
	EndpointWebhook = func(wID int64) string { return EndpointWebhooks + StrID(wID) }
	EndpointWebhookToken = func(wID int64, token string) string { return EndpointWebhooks + StrID(wID) + "/" + token }
	EndpointWebhookMessage = func(wID int64, token, mID string) string {
		return EndpointWebhookToken(wID, token) + "/messages/" + mID
	}

	EndpointDefaultUserAvatar = func(index int) string {
		return EndpointCDN + "embed/avatars/" + strconv.Itoa(index) + ".png"
	}

	EndpointMessageReactionsAll = func(cID, mID int64) string {
		return EndpointChannelMessage(cID, mID) + "/reactions"
	}
	EndpointMessageReactions = func(cID, mID int64, emoji EmojiName) string {
		return EndpointChannelMessage(cID, mID) + "/reactions/" + emoji.String()
	}
	EndpointMessageReaction = func(cID, mID int64, emoji EmojiName, uID string) string {
		return EndpointMessageReactions(cID, mID, emoji) + "/" + uID
	}

	EndpointRelationships = func() string { return EndpointUsers + "@me" + "/relationships" }
	EndpointRelationship = func(uID int64) string { return EndpointRelationships() + "/" + StrID(uID) }
	EndpointRelationshipsMutual = func(uID int64) string { return EndpointUsers + StrID(uID) + "/relationships" }

	EndpointGuildCreate = EndpointAPI + "guilds"

	EndpointInvite = func(iID string) string { return EndpointAPI + "invites/" + iID }

	EndpointIntegrationsJoin = func(iID string) string { return EndpointAPI + "integrations/" + iID + "/join" }

	EndpointEmoji = func(eID int64) string { return EndpointAPI + "emojis/" + StrID(eID) + ".png" }
	EndpointEmojiAnimated = func(eID int64) string { return EndpointAPI + "emojis/" + StrID(eID) + ".gif" }

	EndpointOauth2 = EndpointAPI + "oauth2/"
	EndpointApplications = EndpointOauth2 + "applications"
	EndpointApplication = func(aID int64) string { return EndpointApplications + "/" + StrID(aID) }
	EndpointApplicationMe = EndpointApplications + "/@me"
	EndpointApplicationsBot = func(aID int64) string { return EndpointApplications + "/" + StrID(aID) + "/bot" }

	EndpointApplicationNonOauth2 = func(aID int64) string { return EndpointAPI + "applications/" + StrID(aID) }
	EndpointApplicationCommands = func(aID int64) string { return EndpointApplicationNonOauth2(aID) + "/commands" }
	EndpointApplicationCommand = func(aID int64, cmdID int64) string {
		return EndpointApplicationNonOauth2(aID) + "/commands/" + StrID(cmdID)
	}

	EndpointApplicationGuildCommands = func(aID int64, gID int64) string {
		return EndpointApplicationNonOauth2(aID) + "/guilds/" + StrID(gID) + "/commands"
	}

	EndpointApplicationGuildCommand = func(aID int64, gID int64, cmdID int64) string {
		return EndpointApplicationGuildCommands(aID, gID) + "/" + StrID(cmdID)
	}

	EndpointApplicationGuildCommandsPermissions = func(aID int64, gID int64) string {
		return EndpointApplicationGuildCommands(aID, gID) + "/permissions"
	}

	EndpointApplicationGuildCommandPermissions = func(aID int64, gID int64, cmdID int64) string {
		return EndpointApplicationGuildCommand(aID, gID, cmdID) + "/permissions"
	}

	EndpointSKUs = func(applicationID int64) string {
		return EndpointApplicationNonOauth2(applicationID) + "/skus"
	}
	EndpointEntitlements = func(applicationID int64) string {
		return EndpointApplicationNonOauth2(applicationID) + "/entitlements"
	}
	EndpointEntitlement = func(applicationID, entitlementID int64) string {
		return EndpointApplicationNonOauth2(applicationID) + "/entitlements/" + StrID(entitlementID)
	}
	EndpointEntitlementConsume = func(applicationID, entitlementID int64) string {
		return EndpointApplicationNonOauth2(applicationID) + "/entitlements/" + StrID(entitlementID) + "/consume"
	}
	EndpointSKUSubscriptions = func(skuID int64) string {
		return EndpointAPI + "skus/" + StrID(skuID) + "/subscriptions"
	}
	EndpointSKUSubscription = func(skuID, subscriptionID int64) string {
		return EndpointAPI + "skus/" + StrID(skuID) + "/subscriptions/" + StrID(subscriptionID)
	}

	EndpointInteractions = EndpointAPI + "interactions"
	EndpointInteractionCallback = func(interactionID int64, token string) string {
		return EndpointInteractions + "/" + StrID(interactionID) + "/" + token + "/callback"
	}
	EndpointWebhookInteraction = func(applicationID int64, token string) string {
		return EndpointWebhooks + "/" + StrID(applicationID) + "/" + token
	}
	EndpointInteractionOriginalMessage = func(applicationID int64, token string) string {
		return EndpointWebhookInteraction(applicationID, token) + "/messages/@original"
	}
	EndpointInteractionFollowupMessage = func(applicationID int64, token string, messageID int64) string {
		return EndpointWebhookInteraction(applicationID, token) + "/messages/" + StrID(messageID)
	}
}

func init() {
	CreateEndpoints("https://discord.com/")
}
