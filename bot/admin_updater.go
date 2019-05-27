package bot

import (
	"os"
	"strconv"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
)

var (
	mainServer         int64
	adminRole          int64
	readOnlyAccessRole int64

	// Set of redis admins
	RedisKeyAdmins    = "yagpdb_admins"
	tmpRedisKeyAdmins = "yagpdb_admins_tmp"
	// Set of users with read only access
	RedisKeyReadOnlyAccess    = "yagpdb_ro_access"
	tmpRedisKeyReadOnlyAccess = "yagpdb_ro_access_tmp"
)

func IsBotAdmin(userID int64) (isAdmin bool, err error) {
	if userID == common.Conf.Owner {
		return true, nil
	}

	err = common.RedisPool.Do(retryableredis.FlatCmd(&isAdmin, "SISMEMBER", RedisKeyAdmins, userID))
	return
}

func HasReadOnlyAccess(userID int64) (hasAccess bool, err error) {
	err = common.RedisPool.Do(retryableredis.FlatCmd(&hasAccess, "SISMEMBER", RedisKeyReadOnlyAccess, userID))
	return
}

var stopRunCheckAdmins = make(chan bool)

func loopCheckAdmins() {
	mainServerStr := os.Getenv("YAGPDB_MAIN_SERVER")
	adminRoleStr := os.Getenv("YAGPDB_ADMIN_ROLE")
	readOnlyAccessRoleStr := os.Getenv("YAGPDB_READONLY_ACCESS_ROLE")

	mainServer, _ = strconv.ParseInt(mainServerStr, 10, 64)
	adminRole, _ = strconv.ParseInt(adminRoleStr, 10, 64)
	readOnlyAccessRole, _ = strconv.ParseInt(readOnlyAccessRoleStr, 10, 64)

	if mainServer == 0 || (adminRole == 0 && readOnlyAccessRole == 0) {
		return
	}

	ticker := time.NewTicker(time.Second * 60)
	for {
		select {
		case <-ticker.C:
			if IsGuildOnCurrentProcess(mainServer) {
				requestCheckBotAdmins(mainServer, adminRole, readOnlyAccessRole)
			}
		case <-stopRunCheckAdmins:
			return
		}

	}
}

func requestCheckBotAdmins(mainServer, adminRole, readOnlyRole int64) {
	relevantSession := ShardManager.SessionForGuild(mainServer)
	if relevantSession == nil || relevantSession.GatewayManager.Status() != discordgo.GatewayStatusReady {
		logger.WithField("shard", relevantSession.ShardID).Error("shard not ready, not updating bot admins")
		return
	}

	// Swap the keys updated last round, assuming thats done
	common.RedisPool.Do(retryableredis.Cmd(nil, "RENAME", tmpRedisKeyAdmins, RedisKeyAdmins))
	common.RedisPool.Do(retryableredis.Cmd(nil, "RENAME", tmpRedisKeyReadOnlyAccess, RedisKeyReadOnlyAccess))

	relevantSession.GatewayManager.RequestGuildMembers(mainServer, "", 0)
}

func HandleGuildMembersChunk(data *eventsystem.EventData) {
	evt := data.GuildMembersChunk()

	if evt.GuildID != mainServer {
		return
	}

	for _, member := range evt.Members {
		if adminRole != 0 && common.ContainsInt64Slice(member.Roles, adminRole) {
			err := common.RedisPool.Do(retryableredis.FlatCmd(nil, "SADD", tmpRedisKeyAdmins, member.User.ID))
			if err != nil {
				logger.WithError(err).Error("failed adding user to admins")
			}
		}

		if readOnlyAccessRole != 0 && common.ContainsInt64Slice(member.Roles, readOnlyAccessRole) {
			err := common.RedisPool.Do(retryableredis.FlatCmd(nil, "SADD", tmpRedisKeyReadOnlyAccess, member.User.ID))
			if err != nil {
				logger.WithError(err).Error("failed adding user to read only access users")
			}
		}
	}

}
