package commands

//go:generate sqlboiler --no-hooks -w "commands_channels_overrides,commands_command_overrides" postgres
//REMOVED: generate easyjson  commands.go

import (
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v2/redis"
	log "github.com/sirupsen/logrus"
)

type CtxKey int

const (
	CtxKeyCmdSettings CtxKey = iota
	CtxKeyChannelOverride
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	common.RegisterPlugin(plugin)
	err := common.GORM.AutoMigrate(&common.LoggedExecutedCommand{}).Error
	if err != nil {
		log.WithError(err).Fatal("Failed migrating logged commands database")
	}

	common.ValidateSQLSchema(DBSchema)
	_, err = common.PQ.Exec(DBSchema)
	if err != nil {
		log.WithError(err).Fatal("Failed setting up commands settings tables")
	}
}

func (p *Plugin) Name() string {
	return "Commands"
}

func GetCommandPrefix(client *redis.Client, guild int64) (string, error) {
	reply := client.Cmd("GET", "command_prefix:"+discordgo.StrID(guild))
	if reply.Err != nil {
		return "", reply.Err
	}
	if reply.IsType(redis.Nil) {
		return "", nil
	}

	return reply.Str()
}
