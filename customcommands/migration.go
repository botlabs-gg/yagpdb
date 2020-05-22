package customcommands

import (
	"context"
	"strconv"
	"strings"

	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/boil"
)

// contains stuff for migrating from redis to postgres based configs

func migrateFromRedis() {
	common.RedisPool.Do(radix.WithConn("custom_commands:", func(conn radix.Conn) error {
		scanner := radix.NewScanner(conn, radix.ScanOpts{
			Command: "scan",
			Pattern: "custom_commands:*",
		})

		// san over all the keys
		var key string
		for scanner.Next(&key) {
			// retrieve the guild id from the key
			split := strings.SplitN(key, ":", 2)
			guildID, err := strconv.ParseInt(split[1], 10, 64)
			if err != nil {
				logger.WithError(err).WithField("str", key).Error("custom commands: failed migrating from redis, key is invalid")
				continue
			}

			// perform the migration
			err = migrateGuildConfig(conn, guildID)
			if err != nil {
				logger.WithError(err).WithField("str", key).Error("custom commands: failed migrating from redis")
				continue
			}
		}

		if err := scanner.Close(); err != nil {
			logger.WithError(err).Error("failed scanning keys while migrating custom commands")
			return err
		}

		return nil
	}))
}

func migrateGuildConfig(rc radix.Client, guildID int64) error {
	commands, _, err := LegacyGetCommands(guildID)
	if err != nil || len(commands) < 1 {
		return err
	}

	for _, cmd := range commands {
		localID, err := common.GenLocalIncrID(guildID, "custom_command")
		if err != nil {
			return err
		}

		pqCommand := cmd.ToDBModel()
		pqCommand.GuildID = guildID
		pqCommand.LocalID = localID

		err = pqCommand.InsertG(context.Background(), boil.Infer())
		if err != nil {
			return err
		}

	}

	err = rc.Do(radix.Cmd(nil, "DEL", KeyCommands(guildID)))
	logger.Println("migrated ", len(commands), " custom commands from ", guildID)
	return err
}
