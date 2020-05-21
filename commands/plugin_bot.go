package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/eventsystem"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix/v3"
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
	dcmd.CustomUsernameSearchFunc = p.customUsernameSearchFunc
}

func (p *Plugin) customUsernameSearchFunc(gs *dstate.GuildState, query string) (ms *dstate.MemberState, err error) {
	logger.Info("Searching by username: ", query)
	members, err := bot.BatchMemberJobManager.SearchByUsername(gs.ID, query)
	if err != nil {
		if err == bot.ErrTimeoutWaitingForMember {
			return nil, &dcmd.UserNotFound{query}
		}

		return nil, err
	}

	lowerIn := strings.ToLower(query)

	partialMatches := make([]*discordgo.Member, 0, 5)
	fullMatches := make([]*discordgo.Member, 0, 5)

	// filter out the results
	for _, v := range members {
		if v == nil {
			continue
		}

		if v.User.Username == "" {
			continue
		}

		if strings.EqualFold(query, v.User.Username) || strings.EqualFold(query, v.Nick) {
			fullMatches = append(fullMatches, v)
			if len(fullMatches) >= 5 {
				break
			}
		} else if len(partialMatches) < 5 {
			if strings.Contains(strings.ToLower(v.User.Username), lowerIn) {
				partialMatches = append(partialMatches, v)
			}
		}
	}

	if len(fullMatches) == 1 {
		return dstate.MSFromDGoMember(gs, fullMatches[0]), nil
	}

	if len(fullMatches) == 0 && len(partialMatches) == 0 {
		return nil, &dcmd.UserNotFound{query}
	}

	// Show some help output
	out := ""

	if len(fullMatches)+len(partialMatches) < 10 {
		for _, v := range fullMatches {
			if out != "" {
				out += ", "
			}

			out += "`" + v.User.Username + "`"
		}

		for _, v := range partialMatches {
			if out != "" {
				out += ", "
			}

			out += "`" + v.User.Username + "`"
		}
	} else {
		return nil, &dcmd.UserNotFound{query}
	}

	if len(fullMatches) > 1 {
		return nil, dcmd.NewSimpleUserError("Too many users with the name: (" + out + ") Please re-run the command with a narrower search, mention or ID.")
	}

	return nil, dcmd.NewSimpleUserError("Did you mean one of these? (" + out + ") Please re-run the command with a narrower search, mention or ID")
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

		// Lock the command for execution
		if !BlockingAddRunningCommand(data.Msg.GuildID, data.Msg.ChannelID, data.Msg.Author.ID, yc, time.Second*60) {
			if atomic.LoadInt32(shuttingDown) == 1 {
				return yc.Name + ": Bot is restarting, please try again in a couple seconds...", nil
			}

			return yc.Name + ": Gave up trying to run command after 60 seconds waiting for your previous instance of this command to finish", nil
		}

		defer removeRunningCommand(data.Msg.GuildID, data.Msg.ChannelID, data.Msg.Author.ID, yc)

		// Check if the user can execute the command
		canExecute, resp, settings, err := yc.checkCanExecuteCommand(data, data.CS)
		if err != nil {
			yc.Logger(data).WithError(err).Error("An error occured while checking if we could run command")
		}

		if resp != "" {
			if resp == ReasonCooldown {
				cdLeft, _ := yc.LongestCooldownLeft(data.ContainerChain, data.Msg.Author.ID, data.Msg.GuildID)
				return fmt.Sprintf("This command is on cooldown for another %d seconds", cdLeft), nil
			}

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

func AddRootCommands(p common.Plugin, cmds ...*YAGCommand) {
	for _, v := range cmds {
		v.Plugin = p
		CommandSystem.Root.AddCommand(v, v.GetTrigger())
	}
}

func AddRootCommandsWithMiddlewares(p common.Plugin, middlewares []dcmd.MiddleWareFunc, cmds ...*YAGCommand) {
	for _, v := range cmds {
		v.Plugin = p
		CommandSystem.Root.AddCommand(v, v.GetTrigger().SetMiddlewares(middlewares...))
	}
}

func handleMsgCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()
	if !bot.IsNormalUserMessage(m.Message) {
		// Pls no panicerinos or banerinos self, also ignore webhooks
		return
	}

	abort := false
	for _, filterFunc := range MessageFilterFuncs {
		if !filterFunc(evt, m.Message) {
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

func ensureEmbedLimits(embed *discordgo.MessageEmbed) {
	if utf8.RuneCountInString(embed.Description) < 2000 {
		return
	}

	lines := strings.Split(embed.Description, "\n")

	firstField := &discordgo.MessageEmbedField{
		Name: "Commands",
	}

	currentField := firstField
	for _, v := range lines {
		if utf8.RuneCountInString(currentField.Value)+utf8.RuneCountInString(v) >= 2000 {
			currentField = &discordgo.MessageEmbedField{
				Name:  "...",
				Value: v + "\n",
			}
			embed.Fields = append(embed.Fields, currentField)
		} else {
			currentField.Value += v + "\n"
		}
	}

	embed.Description = firstField.Value
}

func HandleGuildCreate(evt *eventsystem.EventData) {
	g := evt.GuildCreate()

	var prefixExists bool
	err := common.RedisPool.Do(radix.Cmd(&prefixExists, "EXISTS", "command_prefix:"+discordgo.StrID(g.ID)))
	if err != nil {
		logger.WithError(err).Error("Failed checking if prefix exists")
		return
	}

	if !prefixExists {
		defaultPrefix := defaultCommandPrefix()

		common.RedisPool.Do(radix.Cmd(nil, "SET", "command_prefix:"+discordgo.StrID(g.ID), defaultPrefix))
		logger.WithField("guild", g.ID).WithField("g_name", g.Name).Info("Set command prefix to default (" + defaultPrefix + ")")
	}
}

func defaultCommandPrefix() string {
	defaultPrefix := "-"
	if common.Testing {
		defaultPrefix = "("
	}

	return defaultPrefix
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
