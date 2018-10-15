package commands

import (
	"context"
	"fmt"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix.v3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	CommandSystem *dcmd.System
)

var _ bot.BotInitHandler = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandler(HandleGuildCreate, eventsystem.EventGuildCreate)
	eventsystem.AddHandler(handleMsgCreate, eventsystem.EventMessageCreate)

	CommandSystem.State = bot.State
}

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
		err = common.BlockingLockRedisKey(RKeyCommandLock(data.Msg.Author.ID, yc.Name), CommandExecTimeout*2, int((CommandExecTimeout + time.Second).Seconds()))
		if err != nil {
			return nil, errors.WithMessage(err, "Failed locking command")
		}
		defer common.UnlockRedisKey(RKeyCommandLock(data.Msg.Author.ID, yc.Name))

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

func handleMsgCreate(evt *eventsystem.EventData) {
	abort := false
	for _, filterFunc := range MessageFilterFuncs {
		if !filterFunc(evt.MessageCreate().Message) {
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
	err := common.RedisPool.Do(radix.Cmd(&prefixExists, "EXISTS", "command_prefix:"+discordgo.StrID(g.ID)))
	if err != nil {
		log.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		defaultPrefix := "-"
		if common.Testing {
			defaultPrefix = "("
		}

		common.RedisPool.Do(radix.Cmd(nil, "SET", "command_prefix:"+discordgo.StrID(g.ID), defaultPrefix))
		log.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (" + defaultPrefix + ")")
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
