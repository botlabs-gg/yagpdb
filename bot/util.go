package bot

import (
	"context"
	"errors"
	"github.com/bwmarrin/snowflake"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	Cache = cache.New(time.Minute, time.Minute)

	currentStatus       = "v" + common.VERSION + " :)"
	currentStreamingURL = ""
	currentPresenceLock sync.Mutex
)

func init() {
	// Discord epoch
	snowflake.Epoch = 1420070400000
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

func SetStatus(status string) {
	if status == "" {
		status = "v" + common.VERSION + " :)"
	}

	currentPresenceLock.Lock()
	currentStatus = status
	currentPresenceLock.Unlock()

	updateAllShardStatuses()
}

func SetStreaming(streaming, status string) {
	if status == "" {
		status = "v" + common.VERSION + " :)"
	}

	currentPresenceLock.Lock()
	currentStatus = status
	currentStreamingURL = streaming
	currentPresenceLock.Unlock()

	updateAllShardStatuses()
}

func updateAllShardStatuses() {
	currentPresenceLock.Lock()
	stremaing := currentStreamingURL
	status := currentStatus
	currentPresenceLock.Unlock()

	for _, v := range ShardManager.Sessions {
		if stremaing == "" {
			v.UpdateStatus(0, status)
		} else {
			v.UpdateStreamingStatus(0, status, stremaing)
		}
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
		logrus.WithError(err).WithField("guild", gs.ID).Error("Failed checking perms")
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
			logrus.WithError(err).Error("[botrest] failed retrieving total shards")
		}
		<-t.C
	}
}

func fetchTotalShardsFromRedis() error {
	var result int64
	err := common.RedisPool.Do(radix.Cmd(&result, "GET", "yagpdb_total_shards"))
	if err != nil {
		return err
	}

	old := atomic.SwapInt64(redisSetTotalShards, result)
	if old != result {
		logrus.Info("[botrest] new shard count received: ", old, " -> ", result)
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
