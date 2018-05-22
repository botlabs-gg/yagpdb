package commands

import (
	"context"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/templates"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
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
		if tmplCtx.IncreaseCheckCallCounter("commands_user_arg", 2) {
			return nil, errors.New("Max calls to userarg (2) reached")
		}

		if num := templates.ToInt64(v); num != 0 {
			// Assume it's an id
			member, _ := bot.GetMember(tmplCtx.GS.ID(), num)
			if member != nil {
				return member.User, nil
			}

			return nil, errors.New("User not found")
		}

		if str, ok := v.(string); ok {
			// Mention string
			if len(str) < 5 {
				return nil, errors.New("Mention string too short")
			}

			str = strings.TrimSpace(str)

			if strings.HasPrefix(str, "<@") && strings.HasSuffix(str, ">") {
				trimmed := str[2 : len(str)-1]
				if trimmed[0] == '@' {
					trimmed = trimmed[1:]
				}

				id, _ := strconv.ParseInt(trimmed, 10, 64)
				member, _ := bot.GetMember(tmplCtx.GS.ID(), id)
				if member != nil {
					// Found member
					return member.User, nil
				}

			}

			// No more cases we can hanlde
			return nil, errors.New("User not found")
		}

		// Just return whatever we passed
		return v, nil
	}
}

type cmdExecFunc func(cmd string, args ...interface{}) (string, error)

// Returns 2 functions to execute commands in user or bot context with limited about of commands executed
func TmplExecCmdFuncs(ctx *templates.Context, maxExec int, dryRun bool) (userCtxCommandExec cmdExecFunc, botCtxCommandExec cmdExecFunc) {
	execUser := func(cmd string, args ...interface{}) (string, error) {
		if ctx.Redis == nil {
			return "Exec cannot be used here", nil
		}

		mc := &discordgo.MessageCreate{ctx.Msg}
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(ctx, dryRun, ctx.BotUser, mc, cmd, args...)
	}

	execBot := func(cmd string, args ...interface{}) (string, error) {
		if ctx.Redis == nil {
			return "Exec cannot be used here", nil
		}

		botUserCopy := *common.BotUser
		botUserCopy.Username = "YAGPDB (cc: " + ctx.Msg.Author.Username + "#" + ctx.Msg.Author.Discriminator + ")"

		messageCopy := *ctx.Msg
		messageCopy.Author = &botUserCopy

		mc := &discordgo.MessageCreate{&messageCopy}
		if maxExec < 1 {
			return "", errors.New("Max number of commands executed in custom command")
		}
		maxExec -= 1
		return execCmd(ctx, dryRun, ctx.BotUser, mc, cmd, args...)
	}

	return execUser, execBot
}

func execCmd(ctx *templates.Context, dryRun bool, execCtx *discordgo.User, m *discordgo.MessageCreate, cmd string, args ...interface{}) (string, error) {
	fakeMsg := *m.Message
	fakeMsg.Mentions = make([]*discordgo.User, 0)

	cmdLine := cmd + " "

	for _, arg := range args {
		if arg == nil {
			return "", errors.New("Nil arg passed")
		}

		switch t := arg.(type) {
		case string:
			cmdLine += "\"" + t + "\""
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
		default:
			return "", errors.New("Unknown type in exec, contact bot owner")
		}
		cmdLine += " "
	}

	logrus.Info("Custom template is executing a command:", cmdLine)

	fakeMsg.Content = cmdLine

	data, err := CommandSystem.FillData(common.BotSession, &fakeMsg)
	if err != nil {
		return "", errors.WithMessage(err, "tmplExecCmd")
	}
	data.MsgStrippedPrefix = fakeMsg.Content

	foundCmd, rest := CommandSystem.Root.FindCommand(cmdLine)
	if foundCmd == nil {
		return "Unknown command", nil
	}

	data.MsgStrippedPrefix = rest

	data.Cmd = foundCmd

	cast := foundCmd.Command.(*YAGCommand)

	err = dcmd.ParseCmdArgs(data)
	if err != nil {
		return "Failed parsing args", nil
	}

	resp, err := cast.RunFunc(data.WithContext(context.WithValue(data.Context(), CtxKeyRedisClient, ctx.Redis)))
	if err != nil {
		return "", errors.WithMessage(err, "tmplExecCmd, Run")
	}

	// for _, command := range CommandSystem.Commands {
	// 	if !command.CheckMatch(cmdLine, triggerData) {
	// 		continue
	// 	}
	// 	matchedCmd = command
	// 	break
	// }

	// if matchedCmd == nil {
	// 	return "", errors.New("Couldn't find command")
	// }

	// cast, ok := matchedCmd.(*CustomCommand)
	// if !ok {
	// 	return "", errors.New("Unsopported command")
	// }

	// // Do not actually execute the command if it's a dry-run
	// if dryRun {
	// 	return "", nil
	// }

	// parsed, err := cast.ParseCommand(cmdLine, triggerData)
	// if err != nil {
	// 	return "", err
	// }

	// parsed.Source = triggerData.Source
	// parsed.Channel = ctx.CS
	// if ctx.CS == nil {
	// 	parsed.Channel = ctx.GS.Channel(true, ctx.GS.ID())
	// }
	// parsed.Guild = parsed.Channel.Guild

	// resp, err := cast.Run(parsed.WithContext()

	switch v := resp.(type) {
	case error:
		return "Error: " + v.Error(), nil
	case string:
		return v, nil
	case *discordgo.MessageEmbed:
		return common.FallbackEmbed(v), nil
	}

	return "", nil
}
