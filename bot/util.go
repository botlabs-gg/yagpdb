package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"

	"github.com/bwmarrin/snowflake"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate/v3"
	"github.com/jonas747/dutil"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/mediocregopher/radix/v3"
	"github.com/patrickmn/go-cache"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func init() {
	// Discord epoch
	snowflake.Epoch = 1420070400000

	pubsub.FilterFunc = func(guildID int64) (handle bool) {
		if guildID == -1 || ReadyTracker.IsGuildShardReady(guildID) {
			return true
		}

		return false
	}
}

func ContextSession(ctx context.Context) *discordgo.Session {
	return ctx.Value(common.ContextKeyDiscordSession).(*discordgo.Session)
}

func SendDM(user int64, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}

	channel, err := common.BotSession.UserChannelCreate(user)
	if err != nil {
		return err
	}

	_, err = common.BotSession.ChannelMessageSend(channel.ID, msg)
	return err
}

func SendDMEmbed(user int64, embed *discordgo.MessageEmbed) error {
	channel, err := common.BotSession.UserChannelCreate(user)
	if err != nil {
		return err
	}

	_, err = common.BotSession.ChannelMessageSendEmbed(channel.ID, embed)
	return err
}

var (
	ErrStartingUp      = errors.New("Starting up, caches are being filled...")
	ErrGuildNotFound   = errors.New("Guild not found")
	ErrChannelNotFound = errors.New("Channel not found")
)

// AdminOrPerm is the same as AdminOrPermMS but only required a member ID
func AdminOrPerm(guildID int64, channelID int64, userID int64, needed int) (bool, error) {
	// Ensure the member is in state
	ms, err := GetMember(guildID, userID)
	if err != nil {
		return false, err
	}

	return AdminOrPermMS(guildID, channelID, ms, needed)
}

// AdminOrPermMS checks if the provided member has all of the needed permissions or is a admin
func AdminOrPermMS(guildID int64, channelID int64, ms *dstate.MemberState, needed int) (bool, error) {
	guild := State.GetGuild(guildID)
	if guild == nil {
		return false, ErrGuildNotFound
	}

	perms, err := guild.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
	if err != nil {
		return false, err
	}

	if needed != 0 && perms&int64(needed) == int64(needed) {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}

func SnowflakeToTime(i int64) time.Time {
	flake := snowflake.ID(i)
	t := time.Unix(flake.Time()/1000, 0)
	return t
}

func SetStatus(streaming, status string) {
	if status == "" {
		status = "v" + common.VERSION + " :)"
	}

	err1 := common.RedisPool.Do(radix.Cmd(nil, "SET", "status_streaming", streaming))
	err2 := common.RedisPool.Do(radix.Cmd(nil, "SET", "status_name", status))
	if err1 != nil {
		logger.WithError(err1).Error("failed setting bot status in redis")
	}

	if err2 != nil {
		logger.WithError(err2).Error("failed setting bot status in redis")
	}

	pubsub.Publish("bot_status_changed", -1, nil)
}

func updateAllShardStatuses() {
	for _, v := range ShardManager.Sessions {
		RefreshStatus(v)
	}

}

// BotProbablyHasPermission returns true if its possible that the bot has the following permission,
// it also returns true if the bot member could not be found or if the guild is not in state (hence, probably)
func BotHasPermission(guildID int64, channelID int64, permission int) (bool, error) {
	gs := State.GetGuild(guildID)
	if gs == nil {
		return false, ErrGuildNotFound
	}

	return BotHasPermissionGS(gs, channelID, permission)
}

// BotProbablyHasPermissionGS is the same as BotProbablyHasPermission but with a guildstate instead of guildid
func BotHasPermissionGS(gs *dstate.GuildSet, channelID int64, permission int) (bool, error) {
	ms, err := GetMember(gs.ID, common.BotUser.ID)
	if err != nil {
		logger.WithError(err).WithField("guild", gs.ID).Error("bot isnt a member of a guild?")
		return false, err
	}

	perms, err := gs.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
	if err != nil {
		if is, _ := dstate.IsChannelNotFound(err); is {
			// we silently ignore unknown channels
			err = nil
		} else {
			logger.WithError(err).WithField("guild", gs.ID).Error("Failed checking perms")
			return false, err
		}
	}

	if perms&int64(permission) == int64(permission) {
		return true, err
	}

	if perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true, err
	}

	return false, err
}

func BotPermissions(gs *dstate.GuildSet, channelID int64) (int64, error) {
	ms, err := GetMember(gs.ID, common.BotUser.ID)
	if err != nil {
		return 0, err
	}

	perms, err := gs.GetMemberPermissions(channelID, ms.User.ID, ms.Member.Roles)
	if err != nil {
		return 0, err
	}

	return int64(perms), nil
}

func SendMessage(guildID int64, channelID int64, msg string) (permsOK bool, resp *discordgo.Message, err error) {
	hasPerms, err := BotHasPermission(guildID, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages)
	if !hasPerms {
		return false, nil, err
	}

	resp, err = common.BotSession.ChannelMessageSend(channelID, msg)
	permsOK = true
	return
}

func SendMessageGS(gs *dstate.GuildSet, channelID int64, msg string) (permsOK bool, resp *discordgo.Message, err error) {
	hasPerms, err := BotHasPermissionGS(gs, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages)
	if !hasPerms {
		return false, nil, err
	}

	resp, err = common.BotSession.ChannelMessageSend(channelID, msg)
	return true, resp, err
}

func SendMessageEmbed(guildID int64, channelID int64, msg *discordgo.MessageEmbed) (permsOK bool, resp *discordgo.Message, err error) {
	hasPerms, err := BotHasPermission(guildID, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages|discordgo.PermissionEmbedLinks)
	if !hasPerms {
		return false, nil, err
	}

	resp, err = common.BotSession.ChannelMessageSendEmbed(channelID, msg)
	permsOK = true
	return
}

func SendMessageEmbedGS(gs *dstate.GuildSet, channelID int64, msg *discordgo.MessageEmbed) (permsOK bool, resp *discordgo.Message, err error) {
	hasPerms, err := BotHasPermissionGS(gs, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages|discordgo.PermissionEmbedLinks)
	if !hasPerms {
		return false, nil, err
	}

	resp, err = common.BotSession.ChannelMessageSendEmbed(channelID, msg)
	permsOK = true
	return
}

// GuildShardID returns the shard id for the provided guild id
func guildShardID(guildID int64) int {
	return GuildShardID(getTotalShards(), guildID)
}

// GuildShardID returns the shard id for the provided guild id
func GuildShardID(totalShards, guildID int64) int {
	shardID := int((guildID >> 22) % totalShards)
	return shardID
}

func getTotalShards() int64 {
	return int64(totalShardCount)
}

// NodeID returns this node's ID if using the orchestrator system
func NodeID() string {
	if !UsingOrchestrator || NodeConn == nil {
		return "none"
	}

	return NodeConn.GetIDLock()
}

// RefreshStatus updates the provided sessions status according to the current status set
func RefreshStatus(session *discordgo.Session) {
	var streamingURL string
	var status string
	err1 := common.RedisPool.Do(radix.Cmd(&streamingURL, "GET", "status_streaming"))
	err2 := common.RedisPool.Do(radix.Cmd(&status, "GET", "status_name"))
	if err1 != nil {
		logger.WithError(err1).Error("failed retrieiving bot streaming status")
	}
	if err2 != nil {
		logger.WithError(err2).Error("failed retrieiving bot status")
	}

	if streamingURL != "" {
		session.UpdateStreamingStatus(0, status, streamingURL)
	} else {
		session.UpdateStatus(0, status)
	}

}

// IsMemberAbove returns wether ms1 is above ms2 in terms of roles (e.g the highest role of ms1 is higher than the highest role of ms2)
// assumes gs is locked, otherwise race conditions will occur
func IsMemberAbove(gs *dstate.GuildSet, ms1 *dstate.MemberState, ms2 *dstate.MemberState) bool {
	if ms1.User.ID == gs.OwnerID {
		return true
	} else if ms2.User.ID == gs.OwnerID {
		return false
	}

	highestMS1 := MemberHighestRole(gs, ms1)
	highestMS2 := MemberHighestRole(gs, ms2)

	if highestMS1 == nil && highestMS2 == nil {
		// none of them has any roles
		return false
	} else if highestMS1 == nil && highestMS2 != nil {
		// ms1 has no role but ms2 does
		return false
	} else if highestMS1 != nil && highestMS2 == nil {
		// ms1 has a role but not ms2
		return true
	}

	return dutil.IsRoleAbove(highestMS1, highestMS2)
}

// IsMemberAboveRole returns wether ms is above role
// assumes gs is locked, otherwise race conditions will occur
func IsMemberAboveRole(gs *dstate.GuildSet, ms1 *dstate.MemberState, role *discordgo.Role) bool {
	if ms1.User.ID == gs.OwnerID {
		return true
	}

	highestMSRole := MemberHighestRole(gs, ms1)
	if highestMSRole == nil {
		// can't be above the target role when we have no roles
		return false
	}

	return dutil.IsRoleAbove(highestMSRole, role)
}

// MemberHighestRole returns the highest role for ms, assumes gs is rlocked, otherwise race conditions will occur
func MemberHighestRole(gs *dstate.GuildSet, ms *dstate.MemberState) *discordgo.Role {
	var highest *discordgo.Role
	for _, rID := range ms.Member.Roles {
		for _, r := range gs.Roles {
			if r.ID != rID {
				continue
			}

			if highest == nil || dutil.IsRoleAbove(&r, highest) {
				highest = &r
			}

			break
		}
	}

	return highest
}

func GetUsers(guildID int64, ids ...int64) []*discordgo.User {
	resp := make([]*discordgo.User, 0, len(ids))
	for _, id := range ids {
		m := State.GetMember(guildID, id)
		if m != nil {
			resp = append(resp, &m.User)
			continue
		}

		user, err := common.BotSession.User(id)

		if err != nil {
			logger.WithError(err).WithField("guild", guildID).Error("failed retrieving user from api")
			resp = append(resp, &discordgo.User{
				ID:       id,
				Username: "Unknown (" + strconv.FormatInt(id, 10) + ")",
			})
			continue
		}

		resp = append(resp, user)
	}

	return resp
}

type GSCacheKey string

func CheckDiscordErrRetry(err error) bool {
	if err == nil {
		return false
	}

	err = errors.Cause(err)

	if cast, ok := err.(*discordgo.RESTError); ok {
		if cast.Message != nil && cast.Message.Code != 0 {
			// proper discord response, don't retry
			return false
		}
	}

	if err == ErrGuildNotFound {
		return false
	}

	// an unknown error unrelated to the discord api occured (503's for example) attempt a retry
	return true
}

func IsNormalUserMessage(msg *discordgo.Message) bool {
	if msg.Author == nil || msg.Author.ID == common.BotUser.ID || msg.WebhookID != 0 || msg.Author.Discriminator == "0000" || (msg.Member == nil && msg.GuildID != 0) {
		// message edits can have a nil author, those are embed edits
		// check against a discrim of 0000 to avoid some cases on webhook messages where webhook_id is 0, even tough its a webhook
		// discrim is in those 0000 which is a invalid user discrim. (atleast when i was testing)
		return false
	}

	return true
}
