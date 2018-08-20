package bot

import (
	"context"
	"errors"
	"github.com/bwmarrin/snowflake"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/patrickmn/go-cache"
	"strings"
	"sync"
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
