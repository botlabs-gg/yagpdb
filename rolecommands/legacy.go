package rolecommands

import (
	"github.com/Sirupsen/logrus"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
)

func KeyCommands(guildID string) string { return "autorole:" + guildID + ":commands" }

type LegacyRoleCommand struct {
	Role string
	Name string
}

func GetCommandsLegacy(client *redis.Client, guildID string) (roles []*LegacyRoleCommand, err error) {
	err = common.GetRedisJson(client, KeyCommands(guildID), &roles)
	return
}

func (p *Plugin) MigrateStorage(client *redis.Client, guildID int64) error {
	legacyCommands, err := GetCommandsLegacy(client, strconv.FormatInt(guildID, 10))
	if err != nil {
		return err
	}

	for k, cmd := range legacyCommands {

		parsedRole, err := strconv.ParseInt(cmd.Role, 10, 64)
		if err != nil {
			logrus.WithError(err).WithField("guild", guildID).Error("Failed migrating command, could not parse role id: " + cmd.Role)
			continue
		}

		newCommand := &RoleCommand{
			GuildID:  guildID,
			Name:     cmd.Name,
			Role:     parsedRole,
			Position: k,
		}

		err = cmdStore.Insert(newCommand)
		if err != nil {
			return err
		}
	}

	client.Cmd("DEL", KeyCommands(strconv.FormatInt(guildID, 10)))
	return nil
}
