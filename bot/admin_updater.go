package bot

import (
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/config"
	"github.com/mediocregopher/radix/v3"
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
	if common.IsOwner(userID) {
		return true, nil
	}

	err = common.RedisPool.Do(radix.FlatCmd(&isAdmin, "SISMEMBER", RedisKeyAdmins, userID))
	return
}

func HasReadOnlyAccess(userID int64) (hasAccess bool, err error) {
	err = common.RedisPool.Do(radix.FlatCmd(&hasAccess, "SISMEMBER", RedisKeyReadOnlyAccess, userID))
	return
}

var stopRunCheckAdmins = make(chan bool)

var (
	confMainServer         = config.RegisterOption("yagpdb.main.server", "Main server used for various purposes, like assigning people with a certain role as bot admins", int64(0))
	confAdminRole          = config.RegisterOption("yagpdb.admin.role", "People with this role on the main server has bot admin status", int64(0))
	confReadOnlyAccessRole = config.RegisterOption("yagpdb.readonly.access.role", "People with this role on the main server has global read only access to configs", int64(0))
)

func loopCheckAdmins() {
	mainServer = int64(confMainServer.GetInt())
	adminRole = int64(confAdminRole.GetInt())
	readOnlyAccessRole = int64(confReadOnlyAccessRole.GetInt())

	if mainServer == 0 || (adminRole == 0 && readOnlyAccessRole == 0) {
		logger.Info("One of YAGPDB_MAIN_SERVER, YAGPDB_ADMIN_ROLE or YAGPDB_READONLY_ACCESS_ROLE not provided, not running admin checker")
		return
	}
	logger.Info("Admin checker running")

	// always skip rename first iteration, in case the last run had an error
	skipRename := true

	ticker := time.NewTicker(time.Second * 60)
	for {
		select {
		case <-ticker.C:
			if ReadyTracker.IsGuildShardReady(mainServer) {
				err := requestCheckBotAdmins(skipRename, mainServer, adminRole, readOnlyAccessRole)
				if err != nil {
					skipRename = true
					logger.WithError(err).Error("failed updating bot admins")
				} else {
					skipRename = false
				}
			}
		case <-stopRunCheckAdmins:
			return
		}

	}
}

func requestCheckBotAdmins(skipRename bool, mainServer, adminRole, readOnlyRole int64) error {
	logger.Info("checking for admins")

	// Swap the keys updated last round, assuming thats done
	if !skipRename {
		common.RedisPool.Do(radix.Cmd(nil, "RENAME", tmpRedisKeyAdmins, RedisKeyAdmins))
		common.RedisPool.Do(radix.Cmd(nil, "RENAME", tmpRedisKeyReadOnlyAccess, RedisKeyReadOnlyAccess))
	}

	err := BatchMemberJobManager.NewBatchMemberJob(mainServer, func(g int64, members []*discordgo.Member) {
		for _, member := range members {
			if adminRole != 0 && common.ContainsInt64Slice(member.Roles, adminRole) {
				err := common.RedisPool.Do(radix.FlatCmd(nil, "SADD", tmpRedisKeyAdmins, member.User.ID))
				if err != nil {
					logger.WithError(err).Error("failed adding user to admins")
				}
			}

			if readOnlyAccessRole != 0 && common.ContainsInt64Slice(member.Roles, readOnlyAccessRole) {
				err := common.RedisPool.Do(radix.FlatCmd(nil, "SADD", tmpRedisKeyReadOnlyAccess, member.User.ID))
				if err != nil {
					logger.WithError(err).Error("failed adding user to read only access users")
				}
			}
		}
	})

	return err
}

func HandleGuildMembersChunk(data *eventsystem.EventData) {
	go BatchMemberJobManager.handleGuildMemberChunk(data)
}
