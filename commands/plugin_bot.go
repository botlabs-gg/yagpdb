package commands

import (
	"context"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	CommandSystem *dcmd.System
)

func (p *Plugin) InitBot() {
	// Setup the command system
	CommandSystem = &dcmd.System{
		Root: &dcmd.Container{
			HelpTitleEmoji: "ℹ️",
			HelpColor:      0xbeff7a,
			RunInDM:        true,
			IgnoreBots:     true,
		},

		ResponseSender: &dcmd.StdResponseSender{LogErrors: true},
		Prefix:         p,
		State:          bot.State,
	}

	// We have our own middleware before the argument parsing, this is to check for things such as wether the command is enabled at all
	CommandSystem.Root.AddMidlewares(YAGCommandMiddleware, dcmd.ArgParserMW)
	CommandSystem.Root.AddCommand(cmdHelp, cmdHelp.GetTrigger())

	eventsystem.AddHandler(bot.RedisWrapper(HandleGuildCreate), eventsystem.EventGuildCreate)
	eventsystem.AddHandler(handleMsgCreate, eventsystem.EventMessageCreate)
}

func YAGCommandMiddleware(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		client := common.MustGetRedisClient()
		defer common.RedisPool.Put(client)

		yc, ok := data.Cmd.Command.(*YAGCommand)
		if !ok {
			return inner(data)
		}

		// Check if the user can execute the command
		canExecute, resp, settings, err := yc.checkCanExecuteCommand(data, client, data.CS)
		if resp != "" {
			// yc.PostCommandExecuted(settings, data, "", errors.WithMessage(err, "checkCanExecuteCommand"))
			// m, err := common.BotSession.ChannelMessageSend(cState.ID(), resp)
			// go yc.deleteResponse([]*discordgo.Message{m})
			return nil, nil
		}

		if !canExecute {
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		data = data.WithContext(context.WithValue(data.Context(), CtxKeyCmdSettings, settings))

		// Lock the command for execution
		err = common.BlockingLockRedisKey(client, RKeyCommandLock(data.Msg.Author.ID, yc.Name), CommandExecTimeout*2, int((CommandExecTimeout + time.Second).Seconds()))
		if err != nil {
			return nil, errors.WithMessage(err, "Failed locking command")
		}
		defer common.UnlockRedisKey(client, RKeyCommandLock(data.Msg.Author.ID, yc.Name))

		data = data.WithContext(context.WithValue(data.Context(), common.ContextKeyRedis, client))

		innerResp, err := inner(data)

		// Send the response
		yc.PostCommandExecuted(settings, data, innerResp, err)

		return nil, nil
	}
}

func AddRootCommands(cmds ...*YAGCommand) {
	for _, v := range cmds {
		CommandSystem.Root.AddCommand(v, v.GetTrigger())
	}
}

func handleMsgCreate(evt *eventsystem.EventData) {
	CommandSystem.HandleMessageCreate(common.BotSession, evt.MessageCreate)
}

func (p *Plugin) Prefix(data *dcmd.Data) string {
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis connection from pool")
		return ""
	}
	defer common.RedisPool.Put(client)

	prefix, err := GetCommandPrefix(client, data.GS.ID())
	if err != nil {
		log.WithError(err).Error("Failed retrieving commands prefix")
	}

	return prefix
}

var cmdHelp = &YAGCommand{
	Name:        "Help",
	Aliases:     []string{"commands", "h", "how", "command"},
	Description: "Shows help about all or one specific command",
	CmdCategory: CategoryGeneral,
	RunInDM:     true,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "command", Type: dcmd.String},
	},

	RunFunc:  cmdFuncHelp,
	Cooldown: 10,
}

func CmdNotFound(search string) string {
	return fmt.Sprintf("Couldn't find command %q", search)
}

func cmdFuncHelp(data *dcmd.Data) (interface{}, error) {
	target := data.Args[0].Str()

	var resp []*discordgo.MessageEmbed

	// Send the targetted help in the channel it was requested in
	resp = dcmd.GenerateTargettedHelp(target, data, data.ContainerChain[0], &dcmd.StdHelpFormatter{})
	if len(resp) < 1 {
		return CmdNotFound(target), nil
	}

	if len(resp) == 1 {
		// Send short help in same channel
		return resp, nil
	}

	// Send full help in DM
	channel, err := bot.GetCreatePrivateChannel(data.Msg.Author.ID)
	if err != nil {
		return "Something went wrong", err
	}
	for _, v := range resp {
		common.BotSession.ChannelMessageSendEmbed(channel.ID, v)
	}

	return nil, nil
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	client := bot.ContextRedis(evt.Context())
	g := evt.GuildCreate
	prefixExists, err := common.RedisBool(client.Cmd("EXISTS", "command_prefix:"+discordgo.StrID(g.ID)))
	if err != nil {
		log.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		defaultPrefix := "-"
		if common.Testing {
			defaultPrefix = "("
		}

		client.Cmd("SET", "command_prefix:"+discordgo.StrID(g.ID), defaultPrefix)
		log.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (" + defaultPrefix + ")")
	}
}
