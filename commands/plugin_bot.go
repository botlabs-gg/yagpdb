package commands

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
)

var (
	CommandSystem *dcmd.System
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ bot.BotStopperHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, HandleGuildCreate, eventsystem.EventGuildCreate)
	eventsystem.AddHandlerAsyncLastLegacy(p, handleMsgCreate, eventsystem.EventMessageCreate)

	CommandSystem.State = bot.State
}
func (p *Plugin) StopBot(wg *sync.WaitGroup) {
	atomic.StoreInt32(shuttingDown, 1)

	startedWaiting := time.Now()
	for {
		runningcommandsLock.Lock()
		n := len(runningCommands)
		runningcommandsLock.Unlock()

		if n < 1 {
			wg.Done()
			return
		}

		if time.Since(startedWaiting) > time.Second*60 {
			// timeout
			logger.Infof("[commands] timeout waiting for %d commands to finish running (d=%s)", n, time.Since(startedWaiting))
			wg.Done()
			return
		}

		logger.Infof("[commands] waiting for %d commands to finish running (d=%s)", n, time.Since(startedWaiting))
		time.Sleep(time.Millisecond * 500)
	}
}

var helpFormatter = &dcmd.StdHelpFormatter{}

func YAGCommandMiddleware(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		yc, ok := data.Cmd.Command.(*YAGCommand)
		if !ok {
			resp, err := inner(data)
			// Filter the response
			if data.GS != nil {
				if resp == nil && err != nil {
					err = errors.New(FilterResp(err.Error(), data.GS.ID).(string))
				} else if resp != nil {
					resp = FilterResp(resp, data.GS.ID)
				}
			}

			return resp, err
		}

		if data.GS != nil {
			ms, err := bot.GetMember(data.GS.ID, data.Msg.Author.ID)
			if err != nil {
				return nil, errors.WithMessage(err, "failed fetching member")
			}

			data = data.WithContext(context.WithValue(data.Context(), CtxKeyMS, ms))
		}

		// Check if the user can execute the command
		canExecute, resp, settings, err := yc.checkCanExecuteCommand(data, data.CS)
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
		if !BlockingAddRunningCommand(data.Msg.GuildID, data.Msg.ChannelID, data.Msg.Author.ID, yc, time.Second*60) {
			if atomic.LoadInt32(shuttingDown) == 1 {
				return yc.Name + ": Bot is shutting down or restarting, please try again in a couple seconds...", nil
			}

			return yc.Name + ": Gave up trying to run command after 60 seconds waiting for your previous instance of this command to finish", nil
		}

		defer removeRunningCommand(data.Msg.GuildID, data.Msg.ChannelID, data.Msg.Author.ID, yc)

		err = dcmd.ParseCmdArgs(data)
		if err != nil {
			if dcmd.IsUserError(err) {

				args := helpFormatter.ArgDefs(data.Cmd, data)
				switches := helpFormatter.Switches(data.Cmd.Command)

				resp := ""
				if args != "" {
					resp += "```\n" + args + "\n```"
				}
				if switches != "" {
					resp += "```\n" + switches + "\n```"
				}

				resp = resp + "\nInvalid arguments provided: " + err.Error()
				yc.PostCommandExecuted(settings, data, resp, nil)
				return nil, nil
			}

			return nil, err
		}

		innerResp, err := inner(data)

		// Send the response
		yc.PostCommandExecuted(settings, data, innerResp, err)

		return nil, nil
	}
}

func FilterResp(in interface{}, guildID int64) interface{} {
	switch t := in.(type) {
	case string:
		return FilterBadInvites(t, guildID, "[removed-invite]")
	case error:
		return FilterBadInvites(t.Error(), guildID, "[removed-invite]")
	}

	return in
}

func AddRootCommands(cmds ...*YAGCommand) {
	for _, v := range cmds {
		CommandSystem.Root.AddCommand(v, v.GetTrigger())
	}
}
func AddRootCommandsWithMiddlewares(middlewares []dcmd.MiddleWareFunc, cmds ...*YAGCommand) {
	for _, v := range cmds {
		CommandSystem.Root.AddCommand(v, v.GetTrigger().SetMiddlewares(middlewares...))
	}
}

func handleMsgCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()
	if m.Author == nil || m.Author.ID == common.BotUser.ID || m.WebhookID != 0 {
		// Pls no panicerinos or banerinos self, also ignore webhooks
		return
	}

	abort := false
	for _, filterFunc := range MessageFilterFuncs {
		if !filterFunc(m.Message) {
			abort = true
		}
	}

	if abort {
		return
	}

	CommandSystem.HandleMessageCreate(common.BotSession, evt.MessageCreate())
}

func (p *Plugin) Prefix(data *dcmd.Data) string {
	prefix, err := GetCommandPrefix(data.GS.ID)
	if err != nil {
		logger.WithError(err).Error("Failed retrieving commands prefix")
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

	if target != "" {
		if len(resp) != 1 {
			// Send command not found in same channel
			return CmdNotFound(target), nil
		}

		// Send short help in same channel
		return resp, nil
	}

	// Send full help in DM
	channel, err := common.BotSession.UserChannelCreate(data.Msg.Author.ID)
	if err != nil {
		return "Something went wrong, maybe you have DM's disabled? I don't want to spam this channel so here's a external link to available commands: <https://docs.yagpdb.xyz/commands>", err
	}

	for _, v := range resp {
		_, err := common.BotSession.ChannelMessageSendEmbed(channel.ID, v)
		if err != nil {
			return "Something went wrong, maybe you have DM's disabled? I don't want to spam this channel so here's a external link to available commands: <https://docs.yagpdb.xyz/commands>", err
		}
	}

	return "You've got mail!", nil
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	g := evt.GuildCreate()

	var prefixExists bool
	err := common.RedisPool.Do(retryableredis.Cmd(&prefixExists, "EXISTS", "command_prefix:"+discordgo.StrID(g.ID)))
	if err != nil {
		logger.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		defaultPrefix := "-"
		if common.Testing {
			defaultPrefix = "("
		}

		common.RedisPool.Do(retryableredis.Cmd(nil, "SET", "command_prefix:"+discordgo.StrID(g.ID), defaultPrefix))
		logger.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (" + defaultPrefix + ")")
	}
}

var cmdPrefix = &YAGCommand{
	Name:        "Prefix",
	Description: "Shows command prefix of the current server, or the specified server",
	CmdCategory: CategoryTool,
	Arguments: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "Server ID", Type: dcmd.Int, Default: 0},
	},

	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		targetGuildID := data.Args[0].Int64()
		if targetGuildID == 0 {
			targetGuildID = data.GS.ID
		}

		prefix, err := GetCommandPrefix(targetGuildID)
		if err != nil {
			return nil, err
		}

		return fmt.Sprintf("Prefix of `%d`: `%s`", targetGuildID, prefix), nil
	},
}

func ContextMS(ctx context.Context) *dstate.MemberState {
	v := ctx.Value(CtxKeyMS)
	if v == nil {
		return nil
	}

	return v.(*dstate.MemberState)
}
