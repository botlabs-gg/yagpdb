package commands

//go:generate sqlboiler --no-hooks psql
//REMOVED: generate easyjson  commands.go

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	log "github.com/sirupsen/logrus"
)

type CtxKey int

const (
	CtxKeyCmdSettings CtxKey = iota
	CtxKeyChannelOverride
	CtxKeyMS
)

type MessageFilterFunc func(msg *discordgo.Message) bool

// These functions are called on every message, and should return true if the message should be checked for commands, false otherwise
var MessageFilterFuncs []MessageFilterFunc

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

type CommandProvider interface {
	// This is where you should register your commands, called on both the webserver and the bot
	AddCommands()
}

func InitCommands() {
	// Setup the command system
	CommandSystem = &dcmd.System{
		Root: &dcmd.Container{
			HelpTitleEmoji: "ℹ️",
			HelpColor:      0xbeff7a,
			RunInDM:        true,
			IgnoreBots:     true,
		},

		ResponseSender: &dcmd.StdResponseSender{LogErrors: true},
		Prefix:         &Plugin{},
	}

	// We have our own middleware before the argument parsing, this is to check for things such as wether the command is enabled at all
	CommandSystem.Root.AddMidlewares(YAGCommandMiddleware, dcmd.ArgParserMW)
	CommandSystem.Root.AddCommand(cmdHelp, cmdHelp.GetTrigger())
	CommandSystem.Root.AddCommand(cmdPrefix, cmdPrefix.GetTrigger())

	for _, v := range common.Plugins {
		if adder, ok := v.(CommandProvider); ok {
			adder.AddCommands()
		}
	}
}

func (p *Plugin) Name() string {
	return "Commands"
}

func GetCommandPrefix(guild int64) (string, error) {
	var prefix string
	err := common.RedisPool.Do(radix.Cmd(&prefix, "GET", "command_prefix:"+discordgo.StrID(guild)))
	return prefix, err
}
