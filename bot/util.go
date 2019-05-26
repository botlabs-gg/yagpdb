package bot

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/dutil"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/pubsub"
	"github.com/patrickmn/go-cache"
)

var (
	Cache = cache.New(time.Minute, time.Minute)
)

func init() {
	// Discord epoch
	snowflake.Epoch = 1420070400000

	pubsub.FilterFunc = func(guildID int64) (handle bool) {
		if guildID == -1 || IsGuildOnCurrentProcess(guildID) {
			return true
		}

		return false
	}

	pubsub.AddHandler("bot_core_evict_gs_cache", handleEvictCachePubsub, "")
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
	ErrStartingUp    = errors.New("Starting up, caches are being filled...")
	ErrGuildNotFound = errors.New("Guild not found")
)

// AdminOrPerm returns the permissions for the userID in the specified channel
// returns an error if the user or channel is not found
func AdminOrPerm(needed int, userID, channelID int64) (bool, error) {
	channel := State.Channel(true, channelID)
	if channel == nil {
		return false, errors.New("Channel not found")
	}

	// Ensure the member is in state
	GetMember(channel.Guild.ID, userID)
	perms, err := channel.Guild.MemberPermissions(true, channelID, userID)
	if err != nil {
		return false, err
	}

	if perms&needed != 0 {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}

// AdminOrPermMS is the same as AdminOrPerm but with a provided member state
func AdminOrPermMS(ms *dstate.MemberState, channelID int64, needed int) (bool, error) {
	perms, err := ms.Guild.MemberPermissionsMS(true, channelID, ms)
	if err != nil {
		return false, err
	}

	if perms&needed != 0 {
		return true, nil
	}

	if perms&discordgo.PermissionManageServer != 0 || perms&discordgo.PermissionAdministrator != 0 {
		return true, nil
	}

	return false, nil
}

// GuildName is a convenience function for getting the name of a guild
func GuildName(gID int64) (name string) {
	g := State.Guild(true, gID)
	g.RLock()
	name = g.Guild.Name
	g.RUnlock()

	return
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

	err1 := common.RedisPool.Do(retryableredis.Cmd(nil, "SET", "status_streaming", streaming))
	err2 := common.RedisPool.Do(retryableredis.Cmd(nil, "SET", "status_name", status))
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
func BotProbablyHasPermission(guildID int64, channelID int64, permission int) bool {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return true
	}

	return BotProbablyHasPermissionGS(true, gs, channelID, permission)
}

// BotProbablyHasPermissionGS is the same as BotProbablyHasPermission but with a guildstate instead of guildid
func BotProbablyHasPermissionGS(lock bool, gs *dstate.GuildState, channelID int64, permission int) bool {
	perms, err := gs.MemberPermissions(lock, channelID, common.BotUser.ID)
	if err != nil && err != dstate.ErrChannelNotFound {
		logger.WithError(err).WithField("guild", gs.ID).Error("Failed checking perms")
		return true
	}

	if perms&permission == permission {
		return true
	}

	if perms&discordgo.PermissionAdministrator != 0 {
		return true
	}

	return false
}

func SendMessage(guildID int64, channelID int64, msg string) (permsOK bool, resp *discordgo.Message, err error) {
	if !BotProbablyHasPermission(guildID, channelID, discordgo.PermissionSendMessages) {
		return false, nil, nil
	}

	resp, err = common.BotSession.ChannelMessageSend(channelID, msg)
	permsOK = true
	return
}

func SendMessageGS(gs *dstate.GuildState, channelID int64, msg string) (permsOK bool, resp *discordgo.Message, err error) {
	if !BotProbablyHasPermissionGS(true, gs, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages) {
		return false, nil, nil
	}

	resp, err = common.BotSession.ChannelMessageSend(channelID, msg)
	permsOK = true
	return
}

func SendMessageEmbed(guildID int64, channelID int64, msg *discordgo.MessageEmbed) (permsOK bool, resp *discordgo.Message, err error) {
	if !BotProbablyHasPermission(guildID, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages|discordgo.PermissionEmbedLinks) {
		return false, nil, nil
	}

	resp, err = common.BotSession.ChannelMessageSendEmbed(channelID, msg)
	permsOK = true
	return
}

func SendMessageEmbedGS(gs *dstate.GuildState, channelID int64, msg *discordgo.MessageEmbed) (permsOK bool, resp *discordgo.Message, err error) {
	if !BotProbablyHasPermissionGS(true, gs, channelID, discordgo.PermissionSendMessages|discordgo.PermissionReadMessages|discordgo.PermissionEmbedLinks) {
		return false, nil, nil
	}

	resp, err = common.BotSession.ChannelMessageSendEmbed(channelID, msg)
	permsOK = true
	return
}

// IsGuildOnCurrentProcess returns whether the guild is on one of the shards for this process
func IsGuildOnCurrentProcess(guildID int64) bool {
	if !Enabled {
		return false
	}

	processShardsLock.RLock()

	shardID := int((guildID >> 22) % int64(totalShardCount))
	onProcess := common.ContainsIntSlice(processShards, shardID)
	processShardsLock.RUnlock()

	return onProcess
}

// GuildShardID returns the shard id for the provided guild id
func GuildShardID(guildID int64) int {
	totShards := GetTotalShards()

	shardID := int((guildID >> 22) % totShards)
	return shardID
}

var runShardPollerOnce sync.Once

// GetTotalShards either retrieves the total shards from passed command line if the bot is set to run in the same process
// or it starts a background poller to poll redis for it every second
func GetTotalShards() int64 {
	// if the bot is running on this process, then we know the number of total shards
	if Enabled && totalShardCount != 0 {
		return int64(totalShardCount)
	}

	// otherwise we poll it from redis every second
	runShardPollerOnce.Do(func() {
		err := fetchTotalShardsFromRedis()
		if err != nil {
			panic("failed retrieving shards")
		}

		go runNumShardsUpdater()
	})

	return atomic.LoadInt64(redisSetTotalShards)
}

var redisSetTotalShards = new(int64)

func runNumShardsUpdater() {
	t := time.NewTicker(time.Second)
	for {
		err := fetchTotalShardsFromRedis()
		if err != nil {
			logger.WithError(err).Error("[botrest] failed retrieving total shards")
		}
		<-t.C
	}
}

func fetchTotalShardsFromRedis() error {
	var result int64
	err := common.RedisPool.Do(retryableredis.Cmd(&result, "GET", "yagpdb_total_shards"))
	if err != nil {
		return err
	}

	old := atomic.SwapInt64(redisSetTotalShards, result)
	if old != result {
		logger.Info("[botrest] new shard count received: ", old, " -> ", result)
	}

	return nil
}

func GetProcessShards() []int {
	processShardsLock.RLock()

	cop := make([]int, len(processShards))
	copy(cop, processShards)

	processShardsLock.RUnlock()

	return cop
}

func NodeID() string {
	if !UsingOrchestrator || NodeConn == nil {
		return "none"
	}

	return NodeConn.GetIDLock()
}

func RefreshStatus(session *discordgo.Session) {
	var streamingURL string
	var status string
	err1 := common.RedisPool.Do(retryableredis.Cmd(&streamingURL, "GET", "status_streaming"))
	err2 := common.RedisPool.Do(retryableredis.Cmd(&status, "GET", "status_name"))
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
func IsMemberAbove(gs *dstate.GuildState, ms1 *dstate.MemberState, ms2 *dstate.MemberState) bool {
	if ms1.ID == gs.Guild.OwnerID {
		return true
	} else if ms2.ID == gs.Guild.OwnerID {
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
func IsMemberAboveRole(gs *dstate.GuildState, ms1 *dstate.MemberState, role *discordgo.Role) bool {
	if ms1.ID == gs.Guild.OwnerID {
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
func MemberHighestRole(gs *dstate.GuildState, ms *dstate.MemberState) *discordgo.Role {
	var highest *discordgo.Role
	for _, rID := range ms.Roles {
		for _, r := range gs.Guild.Roles {
			if r.ID != rID {
				continue
			}

			if highest == nil || dutil.IsRoleAbove(r, highest) {
				highest = r
			}

			break
		}
	}

	return highest
}

func GetUsers(guildID int64, ids ...int64) []*discordgo.User {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return nil
	}

	return GetUsersGS(gs, ids...)
}

func GetUsersGS(gs *dstate.GuildState, ids ...int64) []*discordgo.User {
	gs.RLock()
	defer gs.RUnlock()

	resp := make([]*discordgo.User, 0, len(ids))
	for _, id := range ids {
		m := gs.Member(false, id)
		if m != nil {
			resp = append(resp, m.DGoUser())
			continue
		}

		gs.RUnlock()

		user, err := common.BotSession.User(id)

		gs.RLock()

		if err != nil {
			logger.WithError(err).WithField("guild", gs.ID).Error("failed retrieving user from api")
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

func EvictGSCache(guildID int64, key GSCacheKey) {
	if Enabled {
		evictGSCacheLocal(guildID, key)
	} else {
		evictGSCacheRemote(guildID, key)
	}
}

func evictGSCacheLocal(guildID int64, key GSCacheKey) {
	gs := State.Guild(true, guildID)
	if gs == nil {
		return
	}

	gs.UserCacheDel(true, key)
}

type GSCacheKey string

func evictGSCacheRemote(guildID int64, key GSCacheKey) {
	err := pubsub.Publish("bot_core_evict_gs_cache", guildID, key)
	if err != nil {
		logger.WithError(err).WithField("guild", guildID).WithField("key", key).Error("failed evicting remote cache")
	}
}

func handleEvictCachePubsub(evt *pubsub.Event) {
	key := evt.Data.(*string)
	evictGSCacheLocal(evt.TargetGuildInt, GSCacheKey(*key))
}
