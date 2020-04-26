package commands

import (
	"context"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/bot/paginatedmessages"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
)

func init() {
	templates.RegisterSetupFunc(func(ctx *templates.Context) {
		execUser, execBot := TmplExecCmdFuncs(ctx, 5, false)
		ctx.ContextFuncs["exec"] = execUser
		ctx.ContextFuncs["execAdmin"] = execBot
		ctx.ContextFuncs["userArg"] = tmplUserArg(ctx)
	})
}

// Returns a user from either id, mention string or if the input is just a user, a user...
func tmplUserArg(tmplCtx *templates.Context) interface{} {
	return func(v interface{}) (interface{}, error) {
		if tmplCtx.IncreaseCheckCallCounter("commands_user_arg", 5) {
			return nil, errors.New("Max calls to userarg (5) reached")
		}

		if num := templates.ToInt64(v); num != 0 {
			// Assume it's an id
			member, _ := bot.GetMember(tmplCtx.GS.ID, num)
			if member != nil {
				return member.DGoUser(), nil
			}

			return nil, nil
		}

		if str, ok := v.(string); ok {
			// Mention string
			if len(str) < 5 {
				return nil, nil
			}

			str = strings.TrimSpace(str)

			if strings.HasPrefix(str, "<@") && strings.HasSuffix(str, ">") {
				trimmed := str[2 : len(str)-1]
				if trimmed[0] == '!' {
					trimmed = trimmed[1:]
				}

				id, _ := strconv.ParseInt(trimmed, 10, 64)
				member, _ := bot.GetMember(tmplCtx.GS.ID, id)
				if member != nil {
					// Found member
					return member.DGoUser(), nil
				}

			}

			// No more cases we can handle
			return nil, nil
		}

		// Just return whatever we passed
		return v, nil
	}
}

type cmdExecFunc func(cmd string, args ...interface{}) (interface{}, error)

// Returns 2 functions to execute commands in user or bot context with limited about of commands executed
func TmplExecCmdFuncs(ctx *templates.Context, maxExec int, dryRun bool) (userCtxCommandExec cmdExecFunc, botCtxCommandExec cmdExecFunc) {
	execUser := func(cmd string, args ...interface{}) (interface{}, error) {
		messageCopy := *ctx.Msg
		if ctx.CurrentFrame.CS != nil { //Check if CS is not a nil pointer
			messageCopy.ChannelID = ctx.CurrentFrame.CS.ID
		}
		mc := &discordgo.MessageCreate{&messageCopy}
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(ctx, dryRun, mc, cmd, args...)
	}

	execBot := func(cmd string, args ...interface{}) (interface{}, error) {

		botUserCopy := *common.BotUser
		botUserCopy.Username = "YAGPDB (cc: " + ctx.Msg.Author.Username + "#" + ctx.Msg.Author.Discriminator + ")"

		messageCopy := *ctx.Msg
		messageCopy.Author = &botUserCopy
		if ctx.CurrentFrame.CS != nil { //Check if CS is not a nil pointer
			messageCopy.ChannelID = ctx.CurrentFrame.CS.ID
		}

		botMember, err := bot.GetMember(messageCopy.GuildID, common.BotUser.ID)
		if err != nil {
			return "", errors.New("Failed fetching member")
		}

		messageCopy.Member = botMember.DGoCopy()

		mc := &discordgo.MessageCreate{&messageCopy}
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(ctx, dryRun, mc, cmd, args...)
	}

	return execUser, execBot
}

func execCmd(tmplCtx *templates.Context, dryRun bool, m *discordgo.MessageCreate, cmd string, args ...interface{}) (interface{}, error) {
	fakeMsg := *m.Message
	fakeMsg.Mentions = make([]*discordgo.User, 0)

	cmdLine := cmd + " "

	for _, arg := range args {
		if arg == nil {
			return "", errors.New("Nil arg passed")
		}

		switch t := arg.(type) {
		case string:
			if strings.HasPrefix(t, "-") {
				// Don't put quotes around switches
				cmdLine += t
			} else if strings.HasPrefix(t, "\\-") {
				// Escaped -
				cmdLine += "\"" + t[1:] + "\""
			} else {
				cmdLine += "\"" + t + "\""
			}
		case int:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int32:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int64:
			cmdLine += strconv.FormatInt(t, 10)
		case uint:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint8:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint16:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint32:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint64:
			cmdLine += strconv.FormatUint(t, 10)
		case float32:
			cmdLine += strconv.FormatFloat(float64(t), 'E', -1, 32)
		case float64:
			cmdLine += strconv.FormatFloat(t, 'E', -1, 64)
		case *discordgo.User:
			cmdLine += "<@" + strconv.FormatInt(t.ID, 10) + ">"
			fakeMsg.Mentions = append(fakeMsg.Mentions, t)
		case []string:
			for i, str := range t {
				if i != 0 {
					cmdLine += " "
				}
				cmdLine += str
			}
		default:
			return "", errors.New("Unknown type in exec, only strings, numbers, users and string slices are supported")
		}
		cmdLine += " "
	}

	logger.Info("Custom template is executing a command:", cmdLine)

	fakeMsg.Content = cmdLine

	data, err := CommandSystem.FillData(common.BotSession, &fakeMsg)
	if err != nil {
		return "", errors.WithMessage(err, "tmplExecCmd")
	}

	data.MsgStrippedPrefix = fakeMsg.Content
	foundCmd, foundContainer, rest := CommandSystem.Root.AbsFindCommandWithRest(cmdLine)
	if foundCmd == nil {
		return "Unknown command", nil
	}

	data.MsgStrippedPrefix = rest

	data.Cmd = foundCmd
	data.ContainerChain = []*dcmd.Container{CommandSystem.Root}
	if foundContainer != CommandSystem.Root {
		data.ContainerChain = append(data.ContainerChain, foundContainer)
	}

	data = data.WithContext(context.WithValue(data.Context(), paginatedmessages.CtxKeyNoPagination, true))

	cast := foundCmd.Command.(*YAGCommand)

	err = dcmd.ParseCmdArgs(data)
	if err != nil {
		return "", errors.WithMessage(err, "exec/execadmin, parseArgs")
	}

	runFunc := cast.RunFunc

	for i := range foundCmd.Trigger.Middlewares {
		runFunc = foundCmd.Trigger.Middlewares[len(foundCmd.Trigger.Middlewares)-1-i](runFunc)
	}

	for i := range data.ContainerChain {
		if i == len(data.ContainerChain)-1 {
			// skip middlewares in original container to bypass cooldowns and stuff
			continue
		}
		runFunc = data.ContainerChain[len(data.ContainerChain)-1-i].BuildMiddlewareChain(runFunc, foundCmd)
	}

	// Check guild scope cooldown
	cd, err := cast.GuildScopeCooldownLeft(data.ContainerChain, tmplCtx.GS.ID)
	if err != nil {
		return "", errors.WithStackIf(err)
	}

	if cd > 0 {
		return "", errors.NewPlain("this command is on guild scope cooldown")
	}

	resp, err := runFunc(data)
	if err != nil {
		return "", errors.WithMessage(err, "exec/execadmin, run")
	}

	cast.SetCooldownGuild(data.ContainerChain, tmplCtx.GS.ID)

	switch v := resp.(type) {
	case error:
		return "Error: " + v.Error(), nil
	case string:
		return v, nil
	case *discordgo.MessageEmbed:
		return v, nil
	case []*discordgo.MessageEmbed:
		return v, nil
	}

	return "", nil
}
